package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/app"
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

	rand.Seed(cli.seed)

	app, err := app.NewApp()
	if err != nil {
		return err
	}
	logger := app.Logger
	logger.
		Info().
		Str("version", toxiproxy.Version).
		Msg("Starting Toxiproxy")

	server := toxiproxy.NewServer(app.Metrics, app.Logger)
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
	err = server.Shutdown()
	if err != nil {
		logger.Err(err).Msg("Shutdown finished with error")
	}
	return nil
}
