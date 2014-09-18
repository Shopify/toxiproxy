package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/Sirupsen/logrus"
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
	collection.Lock()
	defer collection.Unlock()

	for _, proxy := range collection.proxies {
		err := collection.removeByName(proxy.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func (collection *ProxyCollection) AddConfig(path string) {
	// Read the proxies from the JSON configuration file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err":    err,
			"config": path,
		}).Warn("No configuration file loaded")
	} else {
		var configProxies []Proxy

		err := json.Unmarshal(data, &configProxies)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err":    err,
				"config": configPath,
			}).Warn("Unable to unmarshal configuration file")
		}

		for _, proxy := range configProxies {
			// Allocate members since Proxy was created without the initializer
			// (`NewProxy`) which normally takes care of this.
			proxy.allocate()

			err := collection.Add(&proxy)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err":  err,
					"name": proxy.Name,
				}).Warn("Unable to add proxy to collection")
			} else {
				err := proxy.Start()
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"err":      err,
						"name":     proxy.Name,
						"upstream": proxy.Upstream,
						"listen":   proxy.Listen,
					}).Error("Unable to start proxy server")
				}
			}
		}
	}
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
