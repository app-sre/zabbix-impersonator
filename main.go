package main

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	serverListenAddress  string
	serverListenPort     int64
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

			s := NewZServer(&ZServerConfig{
				ServerListenAddress:  serverListenAddress,
				ServerListenPort:     serverListenPort,
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
