package toxiproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
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

	if err := proxy.Validate(); err != nil {
		return err
	}

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

	if err := proxy.Validate(); err != nil {
		return nil, err
	}

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

type populateItem struct {
	proxy        *Proxy
	start        bool
	existing     *Proxy
	keepExisting bool
	skipBind     bool
	listener     net.Listener
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

	items, err := preparePopulateItems(server, input)
	if err != nil {
		return nil, err
	}

	collection.Lock()
	defer collection.Unlock()

	return collection.commitPopulate(items)
}

func preparePopulateItems(
	server *ApiServer,
	input []struct {
		Proxy
		Enabled *bool `json:"enabled"`
	},
) ([]populateItem, error) {
	enabledDefault := true
	for i := range input {
		if len(input[i].Name) < 1 {
			return nil, joinError(fmt.Errorf("name at proxy %d", i+1), ErrMissingField)
		}
		if len(input[i].Upstream) < 1 {
			return nil, joinError(fmt.Errorf("upstream at proxy %d", i+1), ErrMissingField)
		}
		if input[i].Enabled == nil {
			input[i].Enabled = &enabledDefault
		}
	}

	items := make([]populateItem, 0, len(input))
	var errs []error
	for i := range input {
		proxy := NewProxy(server, input[i].Name, input[i].Listen, input[i].Upstream)
		if err := proxy.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("proxy %q: %w", proxy.Name, err))
			continue
		}
		items = append(items, populateItem{
			proxy: proxy,
			start: *input[i].Enabled,
		})
	}
	if len(errs) > 0 {
		return nil, combineErrors(errs)
	}
	return items, nil
}

func (collection *ProxyCollection) commitPopulate(items []populateItem) ([]*Proxy, error) {
	for i := range items {
		existing, exists := collection.proxies[items[i].proxy.Name]
		if !exists {
			continue
		}
		items[i].existing = existing
		differs, err := existing.Differs(items[i].proxy)
		if err != nil {
			return nil, err
		}
		if !differs {
			items[i].keepExisting = true
		}
	}

	reserved := make([]net.Listener, 0)
	started := make([]*Proxy, 0)
	skipBindStopped := make([]*Proxy, 0)
	success := false
	defer func() {
		if success {
			return
		}
		for _, proxy := range started {
			proxy.Stop()
		}
		for _, proxy := range skipBindStopped {
			_ = proxy.Start()
		}
		for _, ln := range reserved {
			if ln != nil {
				ln.Close()
			}
		}
	}()

	for i := range items {
		item := &items[i]
		if item.keepExisting || !item.start {
			continue
		}

		if item.existing != nil && item.existing.Enabled {
			same, err := sameResolvedListen(item.existing.Listen, item.proxy.Listen)
			if err != nil {
				return nil, err
			}
			item.skipBind = same
		}
		if item.skipBind {
			continue
		}

		ln, err := net.Listen("tcp", item.proxy.Listen)
		if err != nil {
			return nil, err
		}
		item.listener = ln
		reserved = append(reserved, ln)
	}

	for i := range items {
		item := &items[i]
		if item.keepExisting || !item.start || item.skipBind || item.listener == nil {
			continue
		}
		item.proxy.listener = item.listener
		for j := range reserved {
			if reserved[j] == item.listener {
				reserved[j] = nil
			}
		}
		item.listener = nil
		if err := item.proxy.Start(); err != nil {
			return nil, err
		}
		started = append(started, item.proxy)
	}

	for i := range items {
		item := &items[i]
		if item.keepExisting || !item.start || !item.skipBind {
			continue
		}
		item.existing.Stop()
		skipBindStopped = append(skipBindStopped, item.existing)
		if err := item.proxy.Start(); err != nil {
			return nil, err
		}
		started = append(started, item.proxy)
	}

	proxies := make([]*Proxy, 0, len(items))
	for i := range items {
		item := &items[i]
		if item.keepExisting {
			proxies = append(proxies, item.existing)
			continue
		}

		if item.existing != nil && !item.skipBind {
			item.existing.Stop()
		}

		collection.proxies[item.proxy.Name] = item.proxy
		proxies = append(proxies, item.proxy)
	}

	success = true
	return proxies, nil
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
