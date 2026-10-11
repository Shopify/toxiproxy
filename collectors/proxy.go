package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
)

type ProxyMetricCollectors struct {
	collectors  []prometheus.Collector
	proxyLabels []string

	ReceivedBytesTotal *prometheus.CounterVec
	SentBytesTotal     *prometheus.CounterVec
}

func (c *ProxyMetricCollectors) Collectors() []prometheus.Collector {
	return c.collectors
}

func NewProxyMetricCollectors() *ProxyMetricCollectors {
	var m ProxyMetricCollectors
	m.proxyLabels = []string{
		"direction",
		"proxy",
		"listener",
		"upstream",
	}
	m.ReceivedBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "proxy",
			Name:      "received_bytes_total",
		},
		m.proxyLabels)
	m.collectors = append(m.collectors, m.ReceivedBytesTotal)

	m.SentBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "proxy",
			Name:      "sent_bytes_total",
		},
		m.proxyLabels)
	m.collectors = append(m.collectors, m.SentBytesTotal)

	return &m
}
