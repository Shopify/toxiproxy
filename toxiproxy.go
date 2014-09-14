package main

import "flag"

var Version = "0.0.1"

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
	proxies.AddConfig(configPath)

	server := NewServer(proxies)
	server.Listen()
}
