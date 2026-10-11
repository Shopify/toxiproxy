package toxiproxy_test

import (
	"flag"
	"net"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/collectors"
	"github.com/Shopify/toxiproxy/v2/testhelper"
)

func NewTestProxy(name, upstream string) *toxiproxy.Proxy {
	log := zerolog.Nop()
	if flag.Lookup("test.v").DefValue == "true" {
		log = zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	}
	srv := toxiproxy.NewServer(
		toxiproxy.NewMetricsContainer(prometheus.NewRegistry()),
		log,
	)
	srv.Metrics.ProxyMetrics = collectors.NewProxyMetricCollectors()
	proxy := toxiproxy.NewProxy(srv, name, "localhost:0", upstream)

	return proxy
}

func WithTCPProxy(
	t *testing.T,
	f func(proxy net.Conn, response chan []byte, proxyServer *toxiproxy.Proxy),
) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()

		conn := AssertProxyUp(t, proxy.Listen, true)

		f(conn, response, proxy)

		proxy.Stop()
	})
}

func AssertProxyUp(t *testing.T, addr string, up bool) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil && up {
		t.Error("Expected proxy to be up:", err)
	} else if err == nil && !up {
		t.Error("Expected proxy to be down")
	}
	return conn
}
