package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type ToxicCollection struct {
	sync.Mutex

	noop   *ToxicWrapper
	proxy  *Proxy
	chain  []*ToxicWrapper
	toxics []*ToxicWrapper
	links  map[string]*ToxicLink
}

func NewToxicCollection(proxy *Proxy) *ToxicCollection {
	collection := &ToxicCollection{
		noop:   &ToxicWrapper{new(NoopToxic), "", "", 0},
		proxy:  proxy,
		chain:  make([]*ToxicWrapper, 1, ToxicCount()+1),
		toxics: make([]*ToxicWrapper, 0, ToxicCount()),
		links:  make(map[string]*ToxicLink),
	}
	collection.chain[0] = collection.noop
	return collection
}

func (c *ToxicCollection) ResetToxics() {
	c.Lock()
	defer c.Unlock()

	for _, toxic := range c.toxics {
		// TODO do this in bulk
		c.chainRemoveToxic(toxic)
	}
	c.toxics = c.toxics[:0]
}

func (c *ToxicCollection) GetToxic(name string) Toxic {
	c.Lock()
	defer c.Unlock()

	for _, toxic := range c.toxics {
		if toxic.Name == name {
			return toxic
		}
	}
	return nil
}

func (c *ToxicCollection) GetToxicMap() map[string]Toxic {
	c.Lock()
	defer c.Unlock()

	result := make(map[string]Toxic)
	for _, toxic := range c.toxics {
		result[toxic.Name] = toxic
	}
	return result
}

func (c *ToxicCollection) AddToxicJson(data io.Reader) (Toxic, error) {
	c.Lock()
	defer c.Unlock()

	var buffer bytes.Buffer

	wrapper := new(ToxicWrapper)
	err := json.NewDecoder(io.TeeReader(data, &buffer)).Decode(wrapper)
	if err != nil {
		return nil, fmt.Errorf("Couldn't decode JSON: %v", err)
	}
	if wrapper.Name == "" {
		wrapper.Name = wrapper.Type
	}

	wrapper.Toxic = NewToxic(wrapper.Type)
	if wrapper.Toxic == nil {
		return nil, fmt.Errorf("Toxic type not found: '%s'", wrapper.Name)
	}

	for _, toxic := range c.toxics {
		if toxic.Name == wrapper.Name {
			return nil, fmt.Errorf("Toxic with same name already exists: '%s'", wrapper.Name)
		}
	}
	err = json.NewDecoder(&buffer).Decode(wrapper.Toxic)
	if err != nil {
		return nil, fmt.Errorf("Couldn't decode toxic JSON: %v", err)
	}

	c.toxics = append(c.toxics, wrapper)
	c.chainAddToxic(wrapper)
	return wrapper.Toxic, nil
}

func (c *ToxicCollection) UpdateToxicJson(name string, data io.Reader) (Toxic, error) {
	c.Lock()
	defer c.Unlock()

	for _, toxic := range c.toxics {
		if toxic.Name == name {
			err := json.NewDecoder(data).Decode(toxic.Toxic)
			if err != nil {
				return nil, err
			}

			c.chainUpdateToxic(toxic)
			return toxic.Toxic, nil
		}
	}
	return nil, fmt.Errorf("Toxic not found: %s", name)
}

func (c *ToxicCollection) RemoveToxic(name string) error {
	c.Lock()
	defer c.Unlock()

	for index, toxic := range c.toxics {
		if toxic.Name == name {
			c.toxics = append(c.toxics[:index], c.toxics[index+1:]...)

			c.chainRemoveToxic(toxic)
			return nil
		}
	}
	return fmt.Errorf("Toxic not found: %s", name)
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

// All following functions assume the lock is already grabbed
func (c *ToxicCollection) chainAddToxic(toxic *ToxicWrapper) {
	toxic.Index = len(c.chain)
	c.chain = append(c.chain, toxic)

	// Asynchronously add the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link *ToxicLink) {
			defer group.Done()
			link.AddToxic(toxic)
		}(link)
	}
	group.Wait()
}

func (c *ToxicCollection) chainUpdateToxic(toxic *ToxicWrapper) {
	c.chain[toxic.Index] = toxic

	// Asynchronously add the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link *ToxicLink) {
			defer group.Done()
			link.UpdateToxic(toxic)
		}(link)
	}
	group.Wait()
}

func (c *ToxicCollection) chainRemoveToxic(toxic *ToxicWrapper) {
	c.chain = append(c.chain[:toxic.Index], c.chain[toxic.Index+1:]...)
	for i := toxic.Index; i < len(c.chain); i++ {
		c.chain[i].Index = i
	}

	// Asynchronously add the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link *ToxicLink) {
			defer group.Done()
			link.RemoveToxic(toxic)
		}(link)
	}
	group.Wait()

	toxic.Index = -1
}
