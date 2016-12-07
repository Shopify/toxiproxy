package main

import (
	"flag"
	"math/rand"
	"os"
	"time"

	"github.com/Shopify/toxiproxy"
	"github.com/Sirupsen/logrus"
)

var host string
var port string
var config string

func init() {
	flag.StringVar(&host, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&port, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.StringVar(&config, "config", "", "JSON file containing proxies to create on startup")
	seed := flag.Int64("seed", time.Now().UTC().UnixNano(), "Seed for randomizing toxics with")
	flag.Parse()
	rand.Seed(*seed)
}

func main() {
	server := toxiproxy.NewServer()
	if len(config) > 0 {
		file, err := os.Open(config)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"config": config,
				"error":  err,
			}).Error("Error reading config file")
		} else {
			proxies, err := server.Collection.PopulateJson(file)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"config": config,
					"error":  err,
				}).Error("Failed to populate proxies from file")
			} else {
				logrus.WithFields(logrus.Fields{
					"config":  config,
					"proxies": len(proxies),
				}).Info("Populated proxies from file")
			}
		}
	}
	server.Listen(host, port)
}
