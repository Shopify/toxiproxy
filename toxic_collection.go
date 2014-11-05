package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	t "github.com/Shopify/toxiproxy/toxics"
)

type ToxicCollection struct {
	sync.Mutex

	noop      *t.NoopToxic
	SlowClose *t.SlowCloseToxic `json:"slow_close"`
	Latency   *t.LatencyToxic   `json:"latency"`
	Timeout   *t.TimeoutToxic   `json:"timeout"`

	proxy  *Proxy
	toxics []t.Toxic
	links  map[string]*ToxicLink
}

func NewToxicCollection(proxy *Proxy) *ToxicCollection {
	collection := &ToxicCollection{
		noop:      new(t.NoopToxic),
		SlowClose: new(t.SlowCloseToxic),
		Latency:   new(t.LatencyToxic),
		Timeout:   new(t.TimeoutToxic),
		proxy:     proxy,
		toxics:    make([]t.Toxic, t.MaxToxics),
		links:     make(map[string]*ToxicLink),
	}
	for i := 0; i < t.MaxToxics; i++ {
		collection.toxics[i] = collection.noop
	}
	return collection
}

func (c *ToxicCollection) NewToxicFromJson(name string, data io.Reader) (t.Toxic, error) {
	var toxic t.Toxic
	switch name {
	case "slow_close":
		temp := *c.SlowClose
		toxic = &temp
	case "latency":
		temp := *c.Latency
		toxic = &temp
	case "timeout":
		temp := *c.Timeout
		toxic = &temp
	default:
		return nil, fmt.Errorf("Bad toxic type: %s", name)
	}
	err := json.NewDecoder(data).Decode(&toxic)
	return toxic, err
}

func (c *ToxicCollection) SetToxic(toxic t.Toxic) error {
	c.Lock()
	defer c.Unlock()

	var index int
	switch v := toxic.(type) {
	case *t.SlowCloseToxic:
		c.SlowClose = v
		index = t.SlowCloseIndex
	case *t.LatencyToxic:
		c.Latency = v
		index = t.LatencyIndex
	case *t.TimeoutToxic:
		c.Timeout = v
		index = t.TimeoutIndex
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
		go func(link *ToxicLink) {
			defer group.Done()
			link.SetToxic(toxic, index)
		}(link)
	}
	group.Wait()
	return nil
}

func (c *ToxicCollection) StartLink(name string, input io.Reader, output io.WriteCloser) {
	c.Lock()
	defer c.Unlock()

	link := NewToxicLink(c.proxy, c)
	link.Start(name, input, output)
	c.links[name] = link
}

func (c *ToxicCollection) RemoveLink(name string) {
	c.Lock()
	defer c.Unlock()
	delete(c.links, name)
}
