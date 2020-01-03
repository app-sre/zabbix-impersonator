package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	connHost = "127.0.0.1"
	connPort = "10051"
	connType = "tcp"
)

var (
	requestsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "processed_requests",
		Help: "The total number of processed zabbix_sender requests",
	})
	requestsInvalid = promauto.NewCounter(prometheus.CounterOpts{
		Name: "invalid_requests",
		Help: "The total number of invalid zabbix_sender requests",
	})
	trapperItemsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "processed_trapper_items",
		Help: "The total number of processed trapper items",
	})
	trapperItemsSkipped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skipped_trapper_items",
		Help: "The total number of skipped trapper items",
	})
)

// TrapperItem TODO
type TrapperItem struct {
	Host    string      `json:"host"`
	FullKey string      `json:"key"`
	Value   interface{} `json:"value"`
}

// Key TODO
func (t TrapperItem) Key() string {
	bracketIndex := strings.Index(t.FullKey, "[")

	if bracketIndex == -1 {
		return t.FullKey
	}

	return t.FullKey[:bracketIndex]
}

// Args TODO
func (t TrapperItem) Args() []string {
	bracketIndex := strings.Index(t.FullKey, "[")

	if bracketIndex == -1 {
		return []string{}
	}

	args := t.FullKey[bracketIndex+1 : len(t.FullKey)-1]
	return strings.Split(args, ",")
}

// ParseFloat64 TODO
func (t TrapperItem) ParseFloat64() (float64, error) {
	var value float64
	var err error

	switch v := t.Value.(type) {
	case string:
		fmt.Println("STRING")
		value, err = strconv.ParseFloat(v, 64)
		if err != nil {
			fmt.Println("ERRSTRING")
			return value, fmt.Errorf("cannot parse %s", v)
		}
	case int:
		value = float64(v)
	case float32:
		value = float64(v)
	case float64:
		value = float64(v)
	default:
		return value, errors.New("invalid type")
	}

	return value, nil
}

// Request TODO
type Request struct {
	Data []TrapperItem `json:"data"`
}

// Metric TODO
type Metric struct {
	ZabbixKey string               `json:"zabbix_key"`
	Metric    string               `json:"metric"`
	Help      string               `json:"help"`
	Args      []string             `json:"args"`
	Gauge     *prometheus.GaugeVec `json:"-"`
}

func sanitizeKey(key string) string {
	return "zabbix_" + strings.Replace(key, ".", "_", -1)
}

func initMetrics() map[string]Metric {
	metricsData, err := ioutil.ReadFile("metrics.json")
	if err != nil {
		panic(err)
	}

	var metricsList []Metric
	err = json.Unmarshal(metricsData, &metricsList)
	if err != nil {
		panic(err)
	}

	var metricsMap = make(map[string]Metric)
	for _, metric := range metricsList {
		if metric.ZabbixKey == "" {
			panic("Unsupported empty ZabbixKey")
		}

		if metric.Metric == "" {
			metric.Metric = sanitizeKey(metric.ZabbixKey)
		}

		metric.Gauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: metric.Metric,
			Help: metric.Help,
		}, append([]string{"host"}, metric.Args...))

		metricsMap[metric.ZabbixKey] = metric
	}

	return metricsMap
}

func zabbixResponse(processed, failed, total int, seconds float64) []byte {
	responseString := fmt.Sprintf(`{"response": "success", "info": "processed: %d; failed: %d; total: %d; seconds spent: %f"}`,
		processed, failed, total, seconds)

	size := make([]byte, 8)
	binary.LittleEndian.PutUint64(size, uint64(len(responseString)))

	buf := bytes.NewBuffer([]byte("ZBXD\x01"))
	buf.Write(size)
	buf.WriteString(responseString)

	return buf.Bytes()
}

var metrics = initMetrics()

func main() {
	// Start prom exporter
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	// Listen for incoming connections.
	l, err := net.Listen(connType, connHost+":"+connPort)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()

	fmt.Println("Listening on " + connHost + ":" + connPort)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {
	defer conn.Close()

	// read header
	respHeader := make([]byte, 13)

	headerLen, err := conn.Read(respHeader)
	if err != nil {
		log.Printf("Error reading header: %s", err.Error())
		requestsInvalid.Inc()
		return
	}

	if headerLen != 13 {
		log.Printf("Incorrect header len")
		requestsInvalid.Inc()
		return
	}

	if !bytes.HasPrefix(respHeader, []byte("ZBXD\x01")) {
		log.Printf("Incorrect header prefix")
		requestsInvalid.Inc()
		return
	}

	bodySize := binary.LittleEndian.Uint64(respHeader[5:])

	respBody := make([]byte, bodySize)
	_, err = conn.Read(respBody)
	if err != nil {
		log.Printf("Error reading body: %s", err.Error())
		requestsInvalid.Inc()
		return
	}

	var request Request
	err = json.Unmarshal(respBody, &request)
	if err != nil {
		log.Printf("Error unmarshalling json: %s", err.Error())
		requestsInvalid.Inc()
		return
	}

	var processed, total int
	for _, trapperItem := range request.Data {
		total++

		metric, ok := metrics[trapperItem.Key()]
		if !ok {
			log.Printf("Skipping unknown metric: %s\n", trapperItem.FullKey)
			trapperItemsSkipped.Inc()
			continue
		}

		// calculate value
		value, err := trapperItem.ParseFloat64()
		if err != nil {
			log.Printf("Skipping metric: %s (%s)", trapperItem.Key(), err.Error())
			trapperItemsSkipped.Inc()
			continue
		}

		labels := append([]string{trapperItem.Host}, trapperItem.Args()...)
		if len(labels) != len(metric.Args)+1 {
			log.Printf("Skipping metric: %s (invalid arg cardinality)", trapperItem.FullKey)
			trapperItemsSkipped.Inc()
			continue
		}

		metric.Gauge.WithLabelValues(labels...).Set(value)
		processed++
		trapperItemsProcessed.Inc()

		log.Printf("[%s] %s (%s) %s: %f\n", trapperItem.Host, metric.Metric, metric.ZabbixKey, trapperItem.Args(), value)
	}

	conn.Write(zabbixResponse(processed, total-processed, total, 0))
	requestsProcessed.Inc()
}
