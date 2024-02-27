package collectors

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMetricsContainer initializes a container for storing all prometheus metrics.
func NewMetricsContainer() *MetricsContainer {
	registry := prometheus.NewRegistry()
	return &MetricsContainer{
		registry: registry,
	}
}

type MetricsContainer struct {
	RuntimeMetrics *RuntimeMetricCollectors
	ProxyMetrics   *ProxyMetricCollectors

	registry *prometheus.Registry
}

func (m *MetricsContainer) RuntimeMetricsEnabled() bool {
	return m.RuntimeMetrics != nil
}

func (m *MetricsContainer) ProxyMetricsEnabled() bool {
	return m.ProxyMetrics != nil
}

// AnyMetricsEnabled determines whether we have any prometheus metrics registered for exporting.
func (m *MetricsContainer) AnyMetricsEnabled() bool {
	return m.RuntimeMetricsEnabled() || m.ProxyMetricsEnabled()
}

// Handler returns an HTTP handler with the necessary collectors registered
// via a global prometheus registry.
func (m *MetricsContainer) Handler() http.Handler {
	if m.RuntimeMetricsEnabled() {
		m.registry.MustRegister(m.RuntimeMetrics.Collectors()...)
	}
	if m.ProxyMetricsEnabled() {
		m.registry.MustRegister(m.ProxyMetrics.Collectors()...)
	}
	return promhttp.HandlerFor(
		m.registry, promhttp.HandlerOpts{Registry: m.registry})
}
