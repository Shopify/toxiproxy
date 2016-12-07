package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/Shopify/toxiproxy"

	"io/ioutil"
	"github.com/Sirupsen/logrus"
	"encoding/json"
)

var host string
var port string

type ProxyConfig struct {
	Name string
	Host   string
	Listen string
}

type ProxyConfigs struct {
	Service [] ProxyConfig
}

func init() {
	flag.StringVar(&host, "host", "localhost", "Host for toxiproxy's API to listen on")
	flag.StringVar(&port, "port", "8474", "Port for toxiproxy's API to listen on")
	seed := flag.Int64("seed", time.Now().UTC().UnixNano(), "Seed for randomizing toxics with")
	flag.Parse()
	rand.Seed(*seed)
}

func main() {
	server := toxiproxy.NewServer()
	addPreConfiguredProxies(server)
	server.Listen(host, port)
}

func addPreConfiguredProxies(server *toxiproxy.ApiServer)  {
	file, e := ioutil.ReadFile("config.json")
	if e != nil {
		logrus.Info("Error reading config file, carry on without.\n", e)
		return
	}
	logrus.Info("Got proxies config:\n", string(file))

	var proxyConfigs ProxyConfigs

	json.Unmarshal(file, &proxyConfigs)

	for _, s := range proxyConfigs.Service {
		logrus.Debug("Adding proxy for upstream %s to listten on \n", s.Host, s.Listen)
		newProxy := toxiproxy.NewProxy();
		newProxy.Name = s.Name
		newProxy.Upstream = s.Host
		newProxy.Listen = s.Listen
		server.Collection.Add(newProxy, true)
	}
}