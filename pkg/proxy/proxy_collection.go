package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/Shopify/toxiproxy/pkg/errors"
)

// Collection is a collection of proxies. It's the interface for anything
// to add and remove proxies from the toxiproxy instance. It's responsibilty is
// to maintain the integrity of the proxy set, by guarding for things such as
// duplicate names.
type Collection struct {
	sync.RWMutex

	proxies map[string]*Proxy
}

func NewCollection() *Collection {
	return &Collection{
		proxies: make(map[string]*Proxy),
	}
}

func (collection *Collection) Add(proxy *Proxy, start bool) error {
	collection.Lock()
	defer collection.Unlock()

	if _, exists := collection.proxies[proxy.Name]; exists {
		return errors.ErrProxyAlreadyExists
	}

	if start {
		err := proxy.Start()
		if err != nil {
			return err
		}
	}

	collection.proxies[proxy.Name] = proxy

	return nil
}

func (collection *Collection) AddOrReplace(proxy *Proxy, start bool) error {
	collection.Lock()
	defer collection.Unlock()

	if existing, exists := collection.proxies[proxy.Name]; exists {
		if existing.Listen == proxy.Listen && existing.Upstream == proxy.Upstream {
			return nil
		}
		existing.Stop()
	}

	if start {
		err := proxy.Start()
		if err != nil {
			return err
		}
	}

	collection.proxies[proxy.Name] = proxy

	return nil
}

func (collection *Collection) PopulateJson(data io.Reader) ([]*Proxy, error) {
	input := []struct {
		Proxy
		Enabled *bool `json:"enabled"` // Overrides Proxy field to make field nullable
	}{}

	err := json.NewDecoder(data).Decode(&input)
	if err != nil {
		return nil, errors.JoinError(err, errors.ErrBadRequestBody)
	}

	// Check for valid input before creating any proxies
	t := true
	for i, p := range input {
		if len(p.Name) < 1 {
			return nil, errors.JoinError(fmt.Errorf("name at proxy %d", i+1), errors.ErrMissingField)
		}
		if len(p.Upstream) < 1 {
			return nil, errors.JoinError(fmt.Errorf("upstream at proxy %d", i+1), errors.ErrMissingField)
		}
		if p.Enabled == nil {
			input[i].Enabled = &t
		}
	}

	proxies := make([]*Proxy, 0, len(input))

	for _, p := range input {
		proxy := NewProxy()
		proxy.Name = p.Name
		proxy.Listen = p.Listen
		proxy.Upstream = p.Upstream

		err = collection.AddOrReplace(proxy, *p.Enabled)
		if err != nil {
			break
		}

		proxies = append(proxies, proxy)
	}
	return proxies, err
}

func (collection *Collection) Proxies() map[string]*Proxy {
	collection.RLock()
	defer collection.RUnlock()

	// Copy the map since using the existing one isn't thread-safe
	proxies := make(map[string]*Proxy, len(collection.proxies))
	for k, v := range collection.proxies {
		proxies[k] = v
	}
	return proxies
}

func (collection *Collection) Get(name string) (*Proxy, error) {
	collection.RLock()
	defer collection.RUnlock()

	return collection.getByName(name)
}

func (collection *Collection) Remove(name string) error {
	collection.Lock()
	defer collection.Unlock()

	proxy, err := collection.getByName(name)
	if err != nil {
		return err
	}
	proxy.Stop()

	delete(collection.proxies, proxy.Name)
	return nil
}

func (collection *Collection) Clear() error {
	collection.Lock()
	defer collection.Unlock()

	for _, proxy := range collection.proxies {
		proxy.Stop()

		delete(collection.proxies, proxy.Name)
	}

	return nil
}

// getByName returns a proxy by its name. Its used from #remove and #get.
// It assumes the lock has already been acquired.
func (collection *Collection) getByName(name string) (*Proxy, error) {
	proxy, exists := collection.proxies[name]
	if !exists {
		return nil, errors.ErrProxyNotFound
	}
	return proxy, nil
}
