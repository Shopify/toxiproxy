package main

import (
	"flag"
	"github.com/Shopify/toxiproxy/metrics"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shopify/toxiproxy"
)

var host string
var port string
var config string
var metricsTimeToKeep string
var metricsMaxEvents int

func init() {
	flag.StringVar(&host, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&port, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.StringVar(&config, "config", "", "JSON file containing proxies to create on startup")
	flag.StringVar(&metricsTimeToKeep, "metrics-time", "10s", "Oldest age of events to keep in toxiproxy metrics (e.g. 20s)")
	flag.IntVar(&metricsMaxEvents, "metrics-max", 100000, "Max num of events to keep in toxiproxy events")
	seed := flag.Int64("seed", time.Now().UTC().UnixNano(), "Seed for randomizing toxics with")
	flag.Parse()
	rand.Seed(*seed)
}

func main() {
	metrics.InitSettings(metricsTimeToKeep, metricsMaxEvents)

	server := toxiproxy.NewServer()
	if len(config) > 0 {
		server.PopulateConfig(config)
	}

	// Handle SIGTERM to exit cleanly
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		<-signals
		os.Exit(0)
	}()

	server.Listen(host, port)
}
