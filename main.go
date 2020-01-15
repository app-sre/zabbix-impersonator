package main

import (
	"net"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	serverListenAddress  string
	serverListenPort     int64
	serverIPWhitelist    []string
	metricsListenAddress string
	metricsListenPort    int64
	metricsFile          string
	metricsNamespace     string
	logLevel             string
	logFormat            string
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "server.listen-address",
				Value:       "0.0.0.0",
				Usage:       "IP for server to listen on",
				EnvVars:     []string{"ZI_SERVER_LISTEN_ADDRESS"},
				Destination: &serverListenAddress,
			},
			&cli.Int64Flag{
				Name:        "server.listen-port",
				Value:       10051,
				Usage:       "port for server to listen on",
				EnvVars:     []string{"ZI_SERVER_LISTEN_PORT"},
				Destination: &serverListenPort,
			},
			&cli.StringSliceFlag{
				Name:        "server.ip-whitelist",
				Value:       cli.NewStringSlice("0.0.0.0/0"),
				Usage:       "IPs that are allowed access",
				EnvVars:     []string{"ZI_SERVER_IP_WHITELIST"},
				DefaultText: "0.0.0.0/0",
			},
			&cli.StringFlag{
				Name:        "metrics.listen-address",
				Value:       "0.0.0.0",
				Usage:       "IP for metrics to listen on",
				EnvVars:     []string{"ZI_METRICS_LISTEN_ADDRESS"},
				Destination: &metricsListenAddress,
			},
			&cli.Int64Flag{
				Name:        "metrics.listen-port",
				Value:       2112,
				Usage:       "port for metrics to listen on",
				EnvVars:     []string{"ZI_METRICS_LISTEN_PORT"},
				Destination: &metricsListenPort,
			},
			&cli.StringFlag{
				Name:        "metrics.file",
				Value:       "metrics.json",
				Usage:       "metrics definition file",
				EnvVars:     []string{"ZI_METRICS_FILE"},
				Destination: &metricsFile,
			},
			&cli.StringFlag{
				Name:        "metrics.namespace",
				Value:       "zabbix_impersonator",
				Usage:       "namespace to expose the metrics under",
				EnvVars:     []string{"ZI_METRICS_NAMESPACE"},
				Destination: &metricsNamespace,
			},
			&cli.StringFlag{
				Name:        "log.level",
				Value:       "info",
				Usage:       "Only log messages with the given severity or above. One of: [debug, info, warn, error]",
				EnvVars:     []string{"ZI_LOG_LEVEL"},
				Destination: &logLevel,
			},
			&cli.StringFlag{
				Name:        "log.format",
				Value:       "text",
				Usage:       "Output format of log messages. One of: [text, json]",
				EnvVars:     []string{"ZI_LOG_FORMAT"},
				Destination: &logFormat,
			},
		},
		Before: func(c *cli.Context) error {
			// StringSliceFlag doesn't support Destination https://github.com/urfave/cli/issues/603
			if len(c.StringSlice("server.ip-whitelist")) > 0 {
				serverIPWhitelist = c.StringSlice("server.ip-whitelist")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			switch strings.ToLower(logLevel) {
			case "debug":
				log.SetLevel(log.DebugLevel)
			case "info":
				log.SetLevel(log.InfoLevel)
			case "warn":
				log.SetLevel(log.WarnLevel)
			case "error":
				log.SetLevel(log.ErrorLevel)
			default:
				log.Fatalf("invalid log level requested: %s", logLevel)
			}

			switch strings.ToLower(logFormat) {
			case "text":
				log.SetFormatter(&log.TextFormatter{
					FullTimestamp: true,
				})
			case "json":
				log.SetFormatter(&log.JSONFormatter{})
			default:
				log.Fatalf("invalid log format requested: %s", logFormat)
			}

			var cidrWhitelist []*net.IPNet
			var ipWhitelist []*net.IP
			for _, iparg := range serverIPWhitelist {
				ips := strings.Split(iparg, ",")
				for _, ip := range ips {
					if strings.Contains(ip, "/") {
						_, ipnet, err := net.ParseCIDR(ip)
						if err != nil {
							log.Fatalf("could not parse CIDR: %v", err)
						}
						cidrWhitelist = append(cidrWhitelist, ipnet)
					} else {
						if parsedIP := net.ParseIP(ip); parsedIP != nil {
							ipWhitelist = append(ipWhitelist, &parsedIP)
						} else {
							log.Fatalf("could not parse IP: %s", ip)
						}
					}

				}
			}

			s := NewZServer(&ZServerConfig{
				ServerListenAddress:  serverListenAddress,
				ServerListenPort:     serverListenPort,
				ServerIPWhitelist:    ipWhitelist,
				ServerCIDRWhitelist:  cidrWhitelist,
				MetricsListenAddress: metricsListenAddress,
				MetricsListenPort:    metricsListenPort,
				MetricsFile:          metricsFile,
				MetricsNamespace:     metricsNamespace,
			})
			return s.Run()
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
