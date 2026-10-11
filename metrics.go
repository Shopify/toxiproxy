package toxiproxy

import (
	"net/http"

	"github.com/Shopify/toxiproxy/v2/collectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMetricsContainer initializes a container for storing all prometheus metrics.
func NewMetricsContainer(registry *prometheus.Registry) *metricsContainer {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	return &metricsContainer{
		registry: registry,
	}
}

type metricsContainer struct {
	RuntimeMetrics *collectors.RuntimeMetricCollectors
	ProxyMetrics   *collectors.ProxyMetricCollectors

	registry *prometheus.Registry
}

func (m *metricsContainer) runtimeMetricsEnabled() bool {
	return m.RuntimeMetrics != nil
}

func (m *metricsContainer) proxyMetricsEnabled() bool {
	return m.ProxyMetrics != nil
}

// anyMetricsEnabled determines whether we have any prometheus metrics registered for exporting.
func (m *metricsContainer) anyMetricsEnabled() bool {
	return m.runtimeMetricsEnabled() || m.proxyMetricsEnabled()
}

// handler returns an HTTP handler with the necessary collectors registered
// via a global prometheus registry.
func (m *metricsContainer) handler() http.Handler {
	if m.runtimeMetricsEnabled() {
		m.registry.MustRegister(m.RuntimeMetrics.Collectors()...)
	}
	if m.proxyMetricsEnabled() {
		m.registry.MustRegister(m.ProxyMetrics.Collectors()...)
	}
	return promhttp.HandlerFor(
		m.registry, promhttp.HandlerOpts{Registry: m.registry})
}
