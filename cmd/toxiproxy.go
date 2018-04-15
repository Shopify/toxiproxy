package main

import (
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shopify/toxiproxy"
	"github.com/sirupsen/logrus"
)

var host string
var port string
var configJson string
var configFile string

func init() {
	flag.StringVar(&host, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&port, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.StringVar(&configFile, "configFile", "", "JSON file containing proxies to create on startup")
	flag.StringVar(&configJson, "configJson", "", "JSON literal containing proxies to create on startup")
	seed := flag.Int64("seed", time.Now().UTC().UnixNano(), "Seed for randomizing toxics with")
	flag.Parse()
	rand.Seed(*seed)
}

func main() {
	server := toxiproxy.NewServer()

	if len(configFile) > 0 && len(configJson) > 0 {
		logrus.WithFields(logrus.Fields{
			"configFile": configFile,
			"configJson": configJson,
		}).Error("configFile and configJson are mutually exclusive")
	} else {
		if len(configFile) > 0 {
			server.PopulateConfigFromFile(configFile)
		}
		if len(configJson) > 0 {
			server.PopulateConfigFromJsonString(configJson)
		}
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
