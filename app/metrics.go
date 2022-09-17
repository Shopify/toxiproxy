package app

import (
	"github.com/Shopify/toxiproxy/v2/collectors"
)

func (a *App) setMetrics() error {
	a.Metrics = collectors.NewMetricsContainer()
	return nil
}
