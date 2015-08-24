package toxiproxy_test

import (
	"testing"

	"github.com/Shopify/toxiproxy/client"
)

var toxiClient *toxiproxy.Client
var redisProxy *toxiproxy.Proxy

func init() {
	var err error
	toxiClient = toxiproxy.NewClient("localhost:8474")
	redisProxy, err = toxiClient.Proxy("redis")
	if err != nil {
		redisProxy, err = toxiClient.CreateProxy("redis", "localhost:26379", "localhost:6379")
		if err != nil {
			panic(err)
		}
	}
}

func TestRedisBackendDown(t *testing.T) {
	redisProxy.Disable()
	defer redisProxy.Enable()

	// Test that redis is down
}

func TestRedisBackendSlow(t *testing.T) {
	redisProxy.AddToxic("", "latency", "", toxiproxy.Toxic{
		"latency": 1000,
	})
	defer redisProxy.RemoveToxic("latency")

	// Test that redis is slow
}
