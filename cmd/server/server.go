package main

import (
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Shopify/toxiproxy/v2"
)

var (
	host   string
	port   string
	config string
)

func init() {
	flag.StringVar(&host, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&port, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.StringVar(&config, "config", "", "JSON file containing proxies to create on startup")
	seed := flag.Int64("seed", time.Now().UTC().UnixNano(), "Seed for randomizing toxics with")
	flag.Parse()
	rand.Seed(*seed)
}

func main() {
	setupLogger()

	server := toxiproxy.NewServer()
	if len(config) > 0 {
		server.PopulateConfig(config)
	}

	// Handle SIGTERM to exit cleanly
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		<-signals
		os.Exit(0)
	}()

	server.Listen(host, port)
}

const LOG_LEVEL_DEFAULT = "info"

func setupLogger() {
	val, ok := os.LookupEnv("LOG_LEVEL")
	level := LOG_LEVEL_DEFAULT
	if ok {
		level = val
	}
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return
	}
	logrus.SetLevel(lvl)
}
