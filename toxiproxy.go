package main

import (
	"flag"
	"math/rand"
	"time"
)

var Version = "0.0.1"

var apiHost string
var apiPort string

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.StringVar(&apiHost, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&apiPort, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.Parse()
}

func main() {
	proxies := NewProxyCollection()
	server := NewServer(proxies)
	server.Listen()
}
