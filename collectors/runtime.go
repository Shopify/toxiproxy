package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type RuntimeMetricCollectors struct {
	collectors []prometheus.Collector
}

func (c *RuntimeMetricCollectors) Collectors() []prometheus.Collector {
	return c.collectors
}

func NewRuntimeMetricCollectors() *RuntimeMetricCollectors {
	var m RuntimeMetricCollectors
	m.collectors = append(m.collectors, collectors.NewGoCollector())
	m.collectors = append(m.collectors, collectors.NewBuildInfoCollector())
	m.collectors = append(m.collectors,
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return &m
}
