package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	log "github.com/sirupsen/logrus"
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

// ZServer defines a zabbix server that will receive trapper requests
type ZServer struct {
	Config  *ZServerConfig
	Metrics map[string]Metric
}

// ZServerConfig defines a ZServer configuration
type ZServerConfig struct {
	ServerListenAddress  string
	ServerListenPort     int64
	MetricsListenAddress string
	MetricsListenPort    int64
	MetricsFile          string
}

// NewZServer instantiates a new ZServer
func NewZServer(c *ZServerConfig) *ZServer {
	return &ZServer{Config: c}
}

// Run starts the ZServer and listens on the server and metrics port
func (s *ZServer) Run() error {
	if err := s.loadMetricsFile(s.Config.MetricsFile); err != nil {
		log.Fatalf("could not load metrics: %v", err)
	}

	// Start prom exporter
	metricsListenIPPort := fmt.Sprintf("%s:%d",
		s.Config.MetricsListenAddress,
		s.Config.MetricsListenPort,
	)
	go func() {
		http.Handle("/metrics", promhttp.Handler())

		log.Infof("Starting metrics server on %s", metricsListenIPPort)
		log.Fatal(http.ListenAndServe(metricsListenIPPort, nil))
	}()

	// Listen for incoming connections.
	serverListenIPPort := fmt.Sprintf("%s:%d",
		s.Config.ServerListenAddress,
		s.Config.ServerListenPort,
	)
	l, err := net.Listen("tcp", serverListenIPPort)
	if err != nil {
		log.Fatalf("could not start listening: %v", err)
	}
	// Close the listener when the application closes.
	defer l.Close()

	log.Infof("Listening for zabbix sender requests on %s", serverListenIPPort)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("error accepting connection: %v", err)
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}

// Handles incoming requests.
func (s *ZServer) handleRequest(conn net.Conn) {
	defer conn.Close()

	// read header
	respHeader := make([]byte, 13)

	headerLen, err := conn.Read(respHeader)
	if err != nil {
		log.Errorf("Error reading header: %s", err.Error())
		requestsInvalid.Inc()
		return
	}

	if headerLen != 13 {
		log.Errorln("Incorrect header len")
		requestsInvalid.Inc()
		return
	}

	if !bytes.HasPrefix(respHeader, []byte("ZBXD\x01")) {
		log.Errorln("Incorrect header prefix")
		requestsInvalid.Inc()
		return
	}

	bodySize := binary.LittleEndian.Uint64(respHeader[5:])

	respBody := make([]byte, bodySize)
	_, err = conn.Read(respBody)
	if err != nil {
		log.Errorf("Error reading body: %s", err.Error())
		requestsInvalid.Inc()
		return
	}

	var request Request
	err = json.Unmarshal(respBody, &request)
	if err != nil {
		log.Errorf("Error unmarshalling json: %s", err.Error())
		requestsInvalid.Inc()
		return
	}

	var processed, total int
	for _, trapperItem := range request.Data {
		total++

		metric, ok := s.Metrics[trapperItem.Key()]
		if !ok {
			log.Warnf("Skipping unknown metric: %s", trapperItem.FullKey)
			trapperItemsSkipped.Inc()
			continue
		}

		// calculate value
		value, err := trapperItem.ParseFloat64()
		if err != nil {
			log.Warnf("Skipping metric: %s (%s)", trapperItem.Key(), err.Error())
			trapperItemsSkipped.Inc()
			continue
		}

		labels := append([]string{trapperItem.Host}, trapperItem.Args()...)
		if len(labels) != len(metric.Args)+1 {
			log.Warnf("Skipping metric: %s (invalid arg cardinality)", trapperItem.FullKey)
			trapperItemsSkipped.Inc()
			continue
		}

		metric.Gauge.WithLabelValues(labels...).Set(value)
		processed++
		trapperItemsProcessed.Inc()

		log.Debugf("[%s] %s (%s) %s: %f\n", trapperItem.Host, metric.Metric, metric.ZabbixKey, trapperItem.Args(), value)
	}

	conn.Write(zabbixResponse(processed, total-processed, total, 0))
	requestsProcessed.Inc()
}

func (s *ZServer) loadMetricsFile(file string) error {
	metricsData, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not read file: %v", err)
	}

	var metricsList []Metric
	err = json.Unmarshal(metricsData, &metricsList)
	if err != nil {
		return fmt.Errorf("could not parse json: %v", err)
	}

	var metricsMap = make(map[string]Metric)
	for _, metric := range metricsList {
		if metric.ZabbixKey == "" {
			return fmt.Errorf("found empty ZabbixKey")
		}

		if metric.Metric == "" {
			metric.Metric = sanitizeKey(metric.ZabbixKey)
		}

		metric.Gauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: metric.Metric,
			Help: metric.Help,
		}, append([]string{"host"}, metric.Args...))

		metricsMap[metric.ZabbixKey] = metric
		log.Infof("Initialized metric %s from zabbix key %s", metric.Metric, metric.ZabbixKey)
	}

	s.Metrics = metricsMap

	return nil
}
