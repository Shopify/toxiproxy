package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/app"
)

func parseArguments() app.ServerOptions {
	result := app.ServerOptions{}
	flag.StringVar(&result.Host, "host", "localhost",
		"Host for toxiproxy's API to listen on")
	flag.StringVar(&result.Port, "port", "8474",
		"Port for toxiproxy's API to listen on")
	flag.StringVar(&result.Config, "config", "",
		"JSON file containing proxies to create on startup")
	flag.Int64Var(&result.Seed, "seed", time.Now().UTC().UnixNano(),
		"Seed for randomizing toxics with")
	flag.BoolVar(&result.RuntimeMetrics, "runtime-metrics", false,
		`enable runtime-related prometheus metrics (default "false")`)
	flag.BoolVar(&result.ProxyMetrics, "proxy-metrics", false,
		`enable toxiproxy-specific prometheus metrics (default "false")`)
	flag.BoolVar(&result.PrintVersion, "version", false,
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
	if cli.PrintVersion {
		fmt.Printf("toxiproxy-server version %s\n", toxiproxy.Version)
		return nil
	}

	app, err := app.NewApp(cli)
	if err != nil {
		return err
	}

	server := toxiproxy.NewServer(app)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func(server *toxiproxy.ApiServer, stop chan os.Signal) {
		err := server.Listen()
		if err != nil {
			server.Logger.Err(err).Msg("Server finished with error")
		}
		close(stop)
	}(server, stop)

	<-stop
	return server.Shutdown()
}
