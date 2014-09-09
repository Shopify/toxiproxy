package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/Sirupsen/logrus"
)

var configPath string
var apiHost string
var apiPort string

func init() {
	flag.StringVar(&configPath, "config", "/etc/toxiproxy.json", "Path to JSON configuration file")
	flag.StringVar(&apiHost, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&apiPort, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.Parse()
}

func main() {
	proxies := NewProxyCollection()

	// Read the proxies from the JSON configuration file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err":    err,
			"config": configPath,
		}).Info("No configuration file loaded")
	} else {
		var configProxies []Proxy

		err := json.Unmarshal(data, &configProxies)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err":    err,
				"config": configPath,
			}).Warn("Unable to unmarshal configuration file")
		}

		for _, proxy := range configProxies {
			// Not allocated since we're not using New
			proxy.started = make(chan bool, 1)

			err := proxies.Add(&proxy)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err":  err,
					"name": proxy.Name,
				}).Warn("Unable to add proxy to collection")
			} else {
				proxy.Start()
			}
		}
	}

	server := NewServer(proxies)
	server.Listen()
}
