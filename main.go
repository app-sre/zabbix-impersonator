package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

var (
	serverListenAddress  string
	serverListenPort     int64
	metricsListenAddress string
	metricsListenPort    int64
	metricsFile          string
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
		},
		Action: func(c *cli.Context) error {
			s := NewZServer(&ZServerConfig{
				ServerListenAddress:  serverListenAddress,
				ServerListenPort:     serverListenPort,
				MetricsListenAddress: metricsListenAddress,
				MetricsListenPort:    metricsListenPort,
				MetricsFile:          metricsFile,
			})
			return s.Run()
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
