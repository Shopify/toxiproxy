package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/Shopify/toxiproxy"
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
		server.PopulateConfig(config)
	}
	server.Listen(host, port)
}
