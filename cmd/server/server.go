package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/collectors"
)

type cliArguments struct {
	host           string
	port           string
	config         string
	seed           int64
	printVersion   bool
	proxyMetrics   bool
	runtimeMetrics bool
}

func parseArguments() cliArguments {
	result := cliArguments{}
	flag.StringVar(&result.host, "host", "localhost",
		"Host for toxiproxy's API to listen on")
	flag.StringVar(&result.port, "port", "8474",
		"Port for toxiproxy's API to listen on")
	flag.StringVar(&result.config, "config", "",
		"JSON file containing proxies to create on startup")
	flag.Int64Var(&result.seed, "seed", time.Now().UTC().UnixNano(),
		"Seed for randomizing toxics with")
	flag.BoolVar(&result.runtimeMetrics, "runtime-metrics", false,
		`enable runtime-related prometheus metrics (default "false")`)
	flag.BoolVar(&result.proxyMetrics, "proxy-metrics", false,
		`enable toxiproxy-specific prometheus metrics (default "false")`)
	flag.BoolVar(&result.printVersion, "version", false,
		`print the version (default "false")`)
	flag.Parse()

	return result
}

func main() {
	err := run()
	if err != nil {
		fmt.Printf("error: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	cli := parseArguments()

	if cli.printVersion {
		fmt.Printf("toxiproxy-server version %s\n", toxiproxy.Version)
		return nil
	}

	seed := cli.seed
	rand.New(rand.NewSource(cli.seed)) // #nosec G404 -- ignoring this rule

	logger := setupLogger()
	log.Logger = logger

	logger.
		Info().
		Str("version", toxiproxy.Version).
		Msg("Starting Toxiproxy")

	metrics := toxiproxy.NewMetricsContainer(prometheus.NewRegistry())
	server := toxiproxy.NewServer(metrics, logger, seed)
	if cli.proxyMetrics {
		server.Metrics.ProxyMetrics = collectors.NewProxyMetricCollectors()
	}
	if cli.runtimeMetrics {
		server.Metrics.RuntimeMetrics = collectors.NewRuntimeMetricCollectors()
	}

	if len(cli.config) > 0 {
		server.PopulateConfig(cli.config)
	}

	addr := net.JoinHostPort(cli.host, cli.port)
	go func(server *toxiproxy.ApiServer, addr string) {
		err := server.Listen(addr)
		if err != nil {
			server.Logger.Err(err).Msg("Server finished with error")
		}
	}(server, addr)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	server.Logger.Info().Msg("Shutdown started")
	err := server.Shutdown()
	if err != nil {
		logger.Err(err).Msg("Shutdown finished with error")
	}
	return nil
}

func setupLogger() zerolog.Logger {
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		return file + ":" + strconv.Itoa(line)
	}

	logger := zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()

	val, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		return logger
	}

	lvl, err := zerolog.ParseLevel(val)
	if err == nil {
		logger = logger.Level(lvl)
	} else {
		l := &logger
		l.Err(err).Msgf("unknown LOG_LEVEL value: \"%s\"", val)
	}

	return logger
}
