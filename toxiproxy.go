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
	var seed int64
	flag.StringVar(&apiHost, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&apiPort, "port", "8474", "Port for toxiproxy's API to listen on")
	flag.Int64Var(&seed, "seed", time.Now().UTC().UnixNano(), "Seed for randomizing toxics with")
	flag.Parse()
	rand.Seed(seed)
}

func main() {
	proxies := NewProxyCollection()
	server := NewServer(proxies)
	server.Listen()
}
