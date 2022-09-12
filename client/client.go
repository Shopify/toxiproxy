// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x
package toxiproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// Client holds information about where to connect to Toxiproxy.
type Client struct {
	endpoint string
}

// NewClient creates a new client which provides the base of all communication
// with Toxiproxy. Endpoint is the address to the proxy (e.g. localhost:8474 if
// not overridden).
func NewClient(endpoint string) *Client {
	if !strings.HasPrefix(endpoint, "https://") && !strings.HasPrefix(endpoint, "http://") {
		endpoint = "http://" + endpoint
	}
	return &Client{endpoint: endpoint}
}

// Proxies returns a map with all the proxies and their toxics.
func (client *Client) Proxies() (map[string]*Proxy, error) {
	resp, err := http.Get(client.endpoint + "/proxies")
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "Proxies")
	if err != nil {
		return nil, err
	}

	proxies := make(map[string]*Proxy)
	err = json.NewDecoder(resp.Body).Decode(&proxies)
	if err != nil {
		return nil, err
	}
	for _, proxy := range proxies {
		proxy.client = client
		proxy.created = true
	}

	return proxies, nil
}

// Generates a new uncommitted proxy instance. In order to use the result, the
// proxy fields will need to be set and have `Save()` called.
func (client *Client) NewProxy() *Proxy {
	return &Proxy{
		client: client,
	}
}

// CreateProxy instantiates a new proxy and starts listening on the specified address.
// This is an alias for `NewProxy()` + `proxy.Save()`.
func (client *Client) CreateProxy(name, listen, upstream string) (*Proxy, error) {
	proxy := &Proxy{
		Name:     name,
		Listen:   listen,
		Upstream: upstream,
		Enabled:  true,
		client:   client,
	}

	err := proxy.Save()
	if err != nil {
		return nil, err
	}

	return proxy, nil
}

// Proxy returns a proxy by name.
func (client *Client) Proxy(name string) (*Proxy, error) {
	// TODO url encode
	resp, err := http.Get(client.endpoint + "/proxies/" + name)
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "Proxy")
	if err != nil {
		return nil, err
	}

	proxy := new(Proxy)
	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return nil, err
	}
	proxy.client = client
	proxy.created = true

	return proxy, nil
}

// Create a list of proxies using a configuration list. If a proxy already exists,
// it will be replaced with the specified configuration.
// For large amounts of proxies, `config` can be loaded from a file.
// Returns a list of the successfully created proxies.
func (client *Client) Populate(config []Proxy) ([]*Proxy, error) {
	proxies := struct {
		Proxies []*Proxy `json:"proxies"`
	}{}
	request, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		client.endpoint+"/populate",
		"application/json",
		bytes.NewReader(request),
	)
	if err != nil {
		return nil, err
	}

	// Response body may need to be read twice, we want to return both the proxy list and any errors
	var body bytes.Buffer
	tee := io.TeeReader(resp.Body, &body)
	err = json.NewDecoder(tee).Decode(&proxies)
	if err != nil {
		return nil, err
	}

	resp.Body = ioutil.NopCloser(&body)
	err = checkError(resp, http.StatusCreated, "Populate")
	if err != nil {
		return proxies.Proxies, err
	}

	for _, proxy := range proxies.Proxies {
		proxy.client = client
	}
	return proxies.Proxies, err
}

// AddToxic creates a toxic to proxy.
func (client *Client) AddToxic(options *ToxicOptions) (*Toxic, error) {
	proxy, err := client.Proxy(options.ProxyName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve proxy with name `%s`: %v", options.ProxyName, err)
	}

	toxic, err := proxy.AddToxic(
		options.ToxicName,
		options.ToxicType,
		options.Stream,
		options.Toxicity,
		options.Attributes,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to add toxic to proxy %s: %v", options.ProxyName, err)
	}

	return toxic, nil
}

// UpdateToxic update a toxic in proxy.
func (client *Client) UpdateToxic(options *ToxicOptions) (*Toxic, error) {
	proxy, err := client.Proxy(options.ProxyName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve proxy with name `%s`: %v", options.ProxyName, err)
	}

	toxic, err := proxy.UpdateToxic(
		options.ToxicName,
		options.Toxicity,
		options.Attributes,
	)

	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to update toxic '%s' of proxy '%s': %v",
				options.ToxicName, options.ProxyName, err,
			)
	}

	return toxic, nil
}

// RemoveToxic removes toxic from proxy.
func (client *Client) RemoveToxic(options *ToxicOptions) error {
	proxy, err := client.Proxy(options.ProxyName)
	if err != nil {
		return fmt.Errorf("failed to retrieve proxy with name `%s`: %v", options.ProxyName, err)
	}

	err = proxy.RemoveToxic(options.ToxicName)
	if err != nil {
		return fmt.Errorf(
			"failed to remove toxic '%s' from proxy '%s': %v",
			options.ToxicName, options.ProxyName, err,
		)
	}

	return nil
}

// ResetState resets the state of all proxies and toxics in Toxiproxy.
func (client *Client) ResetState() error {
	resp, err := http.Post(client.endpoint+"/reset", "text/plain", bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}

	return checkError(resp, http.StatusNoContent, "ResetState")
}
