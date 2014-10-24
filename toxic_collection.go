package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type ToxicCollection struct {
	sync.Mutex

	noop    *NoopToxic
	Latency *LatencyToxic `json:"latency"`

	proxy  *Proxy
	toxics []Toxic
	links  []ToxicLink
}

// Constants used to define which order toxics are chained in.
const (
	LatencyIndex = iota
	MaxToxics
)

func NewToxicCollection(proxy *Proxy) *ToxicCollection {
	collection := &ToxicCollection{
		noop:    new(NoopToxic),
		Latency: new(LatencyToxic),
		proxy:   proxy,
		toxics:  make([]Toxic, MaxToxics),
	}
	for i := 0; i < MaxToxics; i++ {
		collection.toxics[i] = collection.noop
	}
	return collection
}

func (c *ToxicCollection) NewToxicFromJson(name string, data io.Reader) (Toxic, error) {
	var toxic Toxic
	switch name {
	case "latency":
		toxic = &LatencyToxic{c.Latency.Enabled, c.Latency.Latency, c.Latency.Jitter}
	default:
		return nil, fmt.Errorf("Bad toxic type: %s", name)
	}
	err := json.NewDecoder(data).Decode(&toxic)
	return toxic, err
}

func (c *ToxicCollection) SetToxic(toxic Toxic) error {
	c.Lock()
	defer c.Unlock()

	var index int
	switch v := toxic.(type) {
	case *LatencyToxic:
		c.Latency = v
		index = LatencyIndex
	default:
		return fmt.Errorf("Unknown toxic type: %v", toxic)
	}

	if !toxic.IsEnabled() {
		toxic = c.noop
	}
	c.toxics[index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link ToxicLink) {
			defer group.Done()
			link.SetToxic(toxic, index)
		}(link)
	}
	group.Wait()
	return nil
}

func (c *ToxicCollection) StartLink(input io.Reader, output io.WriteCloser) {
	c.Lock()
	defer c.Unlock()

	link := NewToxicLink(c.proxy, input, output)
	link.Start(c.toxics)
	c.links = append(c.links, link)
}
