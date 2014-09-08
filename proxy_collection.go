package main

import (
	"fmt"
	"sync"
)

// ProxyCollection is a collection of proxies. It's the interface for anything
// to add and remove proxies from the toxiproxy instance. It's responsibilty is
// to maintain the integrity of the proxy set, by guarding for things such as
// duplicate names.
type ProxyCollection struct {
	sync.Mutex

	proxies map[string]*Proxy
}

func NewProxyCollection() *ProxyCollection {
	return &ProxyCollection{
		proxies: make(map[string]*Proxy),
	}
}

func (collection *ProxyCollection) Add(proxy *Proxy) error {
	collection.Lock()
	defer collection.Unlock()

	if _, exists := collection.proxies[proxy.Name]; exists {
		return fmt.Errorf("Proxy with name %s already exists", proxy.Name)
	}

	for _, otherProxy := range collection.proxies {
		if proxy.Listen == otherProxy.Listen {
			return fmt.Errorf("Proxy %s is already listening on %s", otherProxy.Name, proxy.Listen)
		}
	}

	collection.proxies[proxy.Name] = proxy

	return nil
}

func (collection *ProxyCollection) Proxies() map[string]*Proxy {
	collection.Lock()
	defer collection.Unlock()

	return collection.proxies
}

func (collection *ProxyCollection) Remove(name string) error {
	collection.Lock()
	defer collection.Unlock()

	return collection.removeByName(name)
}

func (collection *ProxyCollection) Clear() error {
	for _, proxy := range collection.Proxies() {
		err := collection.removeByName(proxy.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

// removeByName removes a proxy by its name. Its used from both #clear and
// #remove. It assumes the lock has already been acquired.
func (collection *ProxyCollection) removeByName(name string) error {
	proxy, exists := collection.proxies[name]
	if !exists {
		return fmt.Errorf("Proxy with name %s doesn't exist", name)
	}

	proxy.Stop()

	delete(collection.proxies, name)

	return nil
}
