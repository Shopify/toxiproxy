package main

import "sync"

type ToxicCollection struct {
	sync.Mutex

	noop              *NoopToxic
	LatencyUpstream   *LatencyToxic `json:"latency_upstream"`
	LatencyDownstream *LatencyToxic `json:"latency_downstream"`

	proxy    *Proxy
	uplink   []Toxic
	downlink []Toxic
}

// Constants used to define which index in link.upToxics / link.downToxics
// each type of toxic uses.
const (
	LatencyIndex = iota
	MaxToxics
)

func NewToxicCollection(proxy *Proxy) *ToxicCollection {
	collection := &ToxicCollection{
		noop:              new(NoopToxic),
		LatencyUpstream:   new(LatencyToxic),
		LatencyDownstream: new(LatencyToxic),
		proxy:             proxy,
		uplink:            make([]Toxic, MaxToxics),
		downlink:          make([]Toxic, MaxToxics),
	}
	for i := 0; i < MaxToxics; i++ {
		collection.uplink[i] = new(NoopToxic)
		collection.downlink[i] = new(NoopToxic)
	}
	return collection
}

func (c *ToxicCollection) SetUpstreamToxic(toxic Toxic, index int) {
	if !toxic.IsEnabled() {
		toxic = c.noop
	}
	c.uplink[index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.proxy.links {
		go func(link *ProxyLink) {
			group.Add(1)
			defer group.Done()
			link.SetUpstreamToxic(toxic, index)
		}(link)
	}
	group.Wait()
}

func (c *ToxicCollection) SetDownstreamToxic(toxic Toxic, index int) {
	if !toxic.IsEnabled() {
		toxic = c.noop
	}
	c.uplink[index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.proxy.links {
		go func(link *ProxyLink) {
			group.Add(1)
			defer group.Done()
			link.SetDownstreamToxic(toxic, index)
		}(link)
	}
	group.Wait()
}
