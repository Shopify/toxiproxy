package toxiproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// ProxyCollection is a collection of proxies. It's the interface for anything
// to add and remove proxies from the toxiproxy instance. It's responsibility is
// to maintain the integrity of the proxy set, by guarding for things such as
// duplicate names.
type ProxyCollection struct {
	sync.RWMutex

	proxies map[string]*Proxy
}

func NewProxyCollection() *ProxyCollection {
	return &ProxyCollection{
		proxies: make(map[string]*Proxy),
	}
}

func (collection *ProxyCollection) Add(proxy *Proxy, start bool) error {
	collection.Lock()
	defer collection.Unlock()

	if _, exists := collection.proxies[proxy.Name]; exists {
		return ErrProxyAlreadyExists
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

func (collection *ProxyCollection) AddOrReplace(proxy *Proxy, start bool) (*Proxy, error) {
	collection.Lock()
	defer collection.Unlock()

	if existing, exists := collection.proxies[proxy.Name]; exists {
		differs, err := existing.Differs(proxy)
		if err != nil {
			return nil, err
		}

		if !differs {
			return existing, nil
		}

		existing.Stop()
	}

	if start {
		err := proxy.Start()
		if err != nil {
			return nil, err
		}
	}

	collection.proxies[proxy.Name] = proxy

	return proxy, nil
}

func (collection *ProxyCollection) PopulateJson(
	server *ApiServer,
	data io.Reader,
) ([]*Proxy, error) {
	input := []struct {
		Proxy
		Enabled *bool `json:"enabled"` // Overrides Proxy field to make field nullable
	}{}

	err := json.NewDecoder(data).Decode(&input)
	if err != nil {
		return nil, joinError(err, ErrBadRequestBody)
	}

	/*
		PHASE 1
		--------
		Validate the complete request before changing the collection.

		This is important because PopulateJson must not partially apply a
		request when a later proxy is invalid.
	*/

	t := true

	proxiesToApply := make([]*Proxy, 0, len(input))
	names := make(map[string]struct{}, len(input))

	for i := range input {
		// Name is required.
		if len(input[i].Name) < 1 {
			return nil, joinError(
				fmt.Errorf("name at proxy %d", i+1),
				ErrMissingField,
			)
		}

		// Upstream is required.
		if len(input[i].Upstream) < 1 {
			return nil, joinError(
				fmt.Errorf("upstream at proxy %d", i+1),
				ErrMissingField,
			)
		}

		// enabled defaults to true when it is omitted.
		if input[i].Enabled == nil {
			input[i].Enabled = &t
		}

		// Do not allow duplicate proxy names in the same request.
		if _, exists := names[input[i].Name]; exists {
			return nil, fmt.Errorf(
				"duplicate proxy name %q at proxy %d",
				input[i].Name,
				i+1,
			)
		}

		names[input[i].Name] = struct{}{}

		proxy := NewProxy(
			server,
			input[i].Name,
			input[i].Listen,
			input[i].Upstream,
		)

		/*
			Only enabled proxies need valid addresses.

			A disabled proxy is allowed to retain an invalid proposed
			listen/upstream address because it does not need to bind a
			listener while disabled.
		*/
		if *input[i].Enabled {
			if err := proxy.Validate(); err != nil {
				return nil, err
			}
		}

		proxiesToApply = append(proxiesToApply, proxy)
	}

	/*
		PHASE 2
		--------
		Save the current collection state before applying anything.

		If a later AddOrReplace fails, we use this state to restore the
		collection and restart the previous proxies.
	*/

	collection.Lock()
	defer collection.Unlock()

	previous := make(map[string]*Proxy, len(collection.proxies))

	for name, proxy := range collection.proxies {
		previous[name] = proxy
	}

	/*
		PHASE 3
		--------
		Apply the complete request.

		We cannot call AddOrReplace here because collection is already locked.
		The logic is therefore performed directly under the same lock.
	*/

	result := make([]*Proxy, 0, len(proxiesToApply))

	for i, proxy := range proxiesToApply {
		start := *input[i].Enabled

		if existing, exists := collection.proxies[proxy.Name]; exists {
			differs, err := existing.Differs(proxy)
			if err != nil {
				collection.rollback(previous)
				return nil, err
			}

			if !differs {
				result = append(result, existing)
				continue
			}

			existing.Stop()
		}

		if start {
			if err := proxy.Start(); err != nil {
				/*
					The current request failed after some changes were
					applied. Restore the complete previous state.
				*/
				proxy.Stop()
				collection.rollback(previous)

				return nil, err
			}
		}

		collection.proxies[proxy.Name] = proxy
		result = append(result, proxy)
	}

	/*
		Remove proxies that existed previously but were not included in
		the new populate request.

		Populate represents the complete proxy set, so old entries not
		present in the submitted array must not remain.
	*/
	for name, proxy := range previous {
		if _, exists := names[name]; !exists {
			proxy.Stop()
			delete(collection.proxies, name)
		}
	}

	return result, nil
}

// rollback restores the proxy collection to the state it had before
// PopulateJson started applying changes.
//
// The collection lock must already be held by the caller.
func (collection *ProxyCollection) rollback(
	previous map[string]*Proxy,
) {
	/*
		Stop and remove all proxies currently present.

		This ensures newly-created/replaced proxies from the failed
		request are no longer part of the serving state.
	*/
	for name, proxy := range collection.proxies {
		if oldProxy, existedBefore := previous[name]; existedBefore && oldProxy == proxy {
			continue
		}

		proxy.Stop()
		delete(collection.proxies, name)
	}

	/*
		Restore the previous proxy objects.

		Restart only proxies that are not currently serving.
	*/
	for name, proxy := range previous {
		current, exists := collection.proxies[name]

		if exists && current == proxy {
			continue
		}

		/*
			Start the old proxy again.

			If the listener cannot be restored, we still restore the
			collection map to the previous object so the in-memory
			collection represents the previous configuration.
		*/
		_ = proxy.Start()

		collection.proxies[name] = proxy
	}
}

func (collection *ProxyCollection) Proxies() map[string]*Proxy {
	collection.RLock()
	defer collection.RUnlock()

	// Copy the map since using the existing one isn't thread-safe
	proxies := make(map[string]*Proxy, len(collection.proxies))
	for k, v := range collection.proxies {
		proxies[k] = v
	}
	return proxies
}

func (collection *ProxyCollection) Get(name string) (*Proxy, error) {
	collection.RLock()
	defer collection.RUnlock()

	return collection.getByName(name)
}

func (collection *ProxyCollection) Remove(name string) error {
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

func (collection *ProxyCollection) Clear() error {
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
func (collection *ProxyCollection) getByName(name string) (*Proxy, error) {
	proxy, exists := collection.proxies[name]
	if !exists {
		return nil, ErrProxyNotFound
	}

	return proxy, nil
}
