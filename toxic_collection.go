package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type ToxicCollection struct {
	sync.Mutex

	noop              *NoopToxic
	LatencyUpstream   *LatencyToxic `json:"latency_upstream"`
	LatencyDownstream *LatencyToxic `json:"latency_downstream"`

	proxy      *Proxy
	upToxics   []Toxic
	downToxics []Toxic
}

// Constants used to define which order toxics are chained in.
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
		upToxics:          make([]Toxic, MaxToxics),
		downToxics:        make([]Toxic, MaxToxics),
	}
	for i := 0; i < MaxToxics; i++ {
		collection.upToxics[i] = new(NoopToxic)
		collection.downToxics[i] = new(NoopToxic)
	}
	return collection
}

func NewToxicFromJson(name string, data io.Reader) (Toxic, error) {
	var toxic Toxic
	switch name {
	case "latency_upstream", "latency_downstream":
		toxic = new(LatencyToxic)
	default:
		return nil, fmt.Errorf("Bad toxic type: %s", name)
	}
	err := json.NewDecoder(data).Decode(&toxic)
	return toxic, err
}

func (c *ToxicCollection) SetUpstreamToxic(toxic Toxic) error {
	c.Lock()
	defer c.Unlock()

	var index int
	switch v := toxic.(type) {
	case *LatencyToxic:
		c.LatencyUpstream = v
		index = LatencyIndex
	default:
		return fmt.Errorf("Unknown toxic type: %v", toxic)
	}

	if !toxic.IsEnabled() {
		toxic = c.noop
	}
	c.upToxics[index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.proxy.uplinks {
		go func(link *ProxyLink) {
			group.Add(1)
			defer group.Done()
			link.SetToxic(toxic, index)
		}(link)
	}
	group.Wait()
	return nil
}

func (c *ToxicCollection) SetDownstreamToxic(toxic Toxic) error {
	c.Lock()
	defer c.Unlock()

	var index int
	switch v := toxic.(type) {
	case *LatencyToxic:
		c.LatencyDownstream = v
		index = LatencyIndex
	default:
		return fmt.Errorf("Unknown toxic type: %v", toxic)
	}

	if !toxic.IsEnabled() {
		toxic = c.noop
	}
	c.downToxics[index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.proxy.downlinks {
		go func(link *ProxyLink) {
			group.Add(1)
			defer group.Done()
			link.SetToxic(toxic, index)
		}(link)
	}
	group.Wait()
	return nil
}
