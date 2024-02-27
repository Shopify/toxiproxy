package app

import (
	"github.com/Shopify/toxiproxy/v2/collectors"
)

func (a *App) setMetrics() error {
	metrics := collectors.NewMetricsContainer()
	if a.EnabledProxyMetrics {
		metrics.ProxyMetrics = collectors.NewProxyMetricCollectors()
	}
	if a.EnabledRuntimeMetrics {
		metrics.RuntimeMetrics = collectors.NewRuntimeMetricCollectors()
	}
	a.Metrics = metrics
	return nil
}
