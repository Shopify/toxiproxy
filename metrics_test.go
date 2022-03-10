package toxiproxy

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Shopify/toxiproxy/v2/collectors"
	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/prometheus/client_golang/prometheus"
)

func TestProxyMetricsReceivedSentBytes(t *testing.T) {
	expectedMetrics := []string{

		`toxiproxy_proxy_received_bytes_total{direction="upstream",listener="localhost:0",proxy="test",upstream="upstream"} 5`, //nolint
		`toxiproxy_proxy_sent_bytes_total{direction="upstream",listener="localhost:0",proxy="test",upstream="upstream"} 5`,     //nolint
	}
	srv := NewServer(NewMetricsContainer(prometheus.NewRegistry()))
	srv.Metrics.ProxyMetrics = collectors.NewProxyMetricCollectors()
	proxy := NewProxy(srv)
	proxy.Name = "test"
	proxy.Listen = "localhost:0"
	proxy.Upstream = "upstream"
	r := bufio.NewReader(bytes.NewBufferString("hello"))
	w := &testWriteCloser{
		bufio.NewWriter(bytes.NewBuffer([]byte{})),
	}
	linkName := "testupstream"
	proxy.Toxics.StartLink(srv, linkName, r, w, stream.Upstream)
	proxy.Toxics.RemoveLink(linkName)
	gotMetrics := prometheusOutput(t, srv, "toxiproxy")
	if !reflect.DeepEqual(gotMetrics, expectedMetrics) {
		t.Fatalf("expected: %v got: %v", expectedMetrics, gotMetrics)
	}
}
func TestRuntimeMetricsBuildInfo(t *testing.T) {
	expectedMetrics := []string{
		`go_build_info{checksum="unknown",path="unknown",version="unknown"} 1`,
	}
	srv := NewServer(NewMetricsContainer(prometheus.NewRegistry()))
	srv.Metrics.RuntimeMetrics = collectors.NewRuntimeMetricCollectors()

	gotMetrics := prometheusOutput(t, srv, "go_build_info")
	if !reflect.DeepEqual(gotMetrics, expectedMetrics) {
		t.Fatalf("expected: %v got: %v", expectedMetrics, gotMetrics)
	}
}

type testWriteCloser struct {
	*bufio.Writer
}

func (t *testWriteCloser) Close() error {
	return t.Flush()
}

func prometheusOutput(t *testing.T, apiServer *ApiServer, prefix string) []string {
	t.Helper()

	testServer := httptest.NewServer(apiServer.Metrics.handler())
	defer testServer.Close()
	resp, err := http.Get(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var selected []string
	s := bufio.NewScanner(resp.Body)
	for s.Scan() {
		if strings.HasPrefix(s.Text(), prefix) {
			selected = append(selected, s.Text())
		}
	}
	return selected
}
