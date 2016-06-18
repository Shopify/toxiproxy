// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x
package toxiproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Client holds information about where to connect to Toxiproxy.
type Client struct {
	endpoint string
}

type Attributes map[string]interface{}

type Toxic struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Stream     string     `json:"stream,omitempty"`
	Toxicity   float32    `json:"toxicity"`
	Attributes Attributes `json:"attributes"`
}

type Toxics []Toxic

// Proxy represents a Proxy.
type Proxy struct {
	Name     string `json:"name"`     // The name of the proxy
	Listen   string `json:"listen"`   // The address the proxy listens on
	Upstream string `json:"upstream"` // The upstream address to proxy to
	Enabled  bool   `json:"enabled"`  // Whether the proxy is enabled

	ActiveToxics Toxics `json:"toxics"` // The toxics active on this proxy

	client  *Client
	created bool // True if this proxy exists on the server
}

// NewClient creates a new client which provides the base of all communication
// with Toxiproxy. Endpoint is the address to the proxy (e.g. localhost:8474 if
// not overriden)
func NewClient(endpoint string) *Client {
	if !strings.HasPrefix(endpoint, "http://") {
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
// This is an alias for `NewProxy()` + `proxy.Save()`
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

// Create a list of proxies using a configuration list. If a proxy already exists, it will be replaced
// with the specified configuration. For large amounts of proxies, `config` can be loaded from a file.
func (client *Client) Populate(config []Proxy) (map[string]*Proxy, error) {
	proxies := make(map[string]*Proxy, len(config))
	for _, proxy := range config {
		existing, err := client.Proxy(proxy.Name)
		if err != nil && err.Error() != "Proxy: HTTP 404: proxy not found" {
			return nil, err
		} else if existing != nil && (existing.Listen != proxy.Listen || existing.Upstream != proxy.Upstream) {
			existing.Delete()
		}
		proxies[proxy.Name], err = client.CreateProxy(proxy.Name, proxy.Listen, proxy.Upstream)
		if err != nil {
			return nil, err
		}
	}
	return proxies, nil
}

// TODO(jpittis) replace this with the old populate command.
func (client *Client) Populate2(config []Proxy) ([]*Proxy, error) {
	proxies := make([]*Proxy, len(config))
	request, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(client.endpoint+"/populate", "application/json", bytes.NewReader(request))
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusCreated, "Create")
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(resp.Body).Decode(&proxies)
	if err != nil {
		return nil, err
	}

	return proxies, nil
}

// Save saves changes to a proxy such as its enabled status or upstream port.
func (proxy *Proxy) Save() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}

	var resp *http.Response
	if proxy.created {
		resp, err = http.Post(proxy.client.endpoint+"/proxies/"+proxy.Name, "text/plain", bytes.NewReader(request))
	} else {
		resp, err = http.Post(proxy.client.endpoint+"/proxies", "application/json", bytes.NewReader(request))
	}
	if err != nil {
		return err
	}

	if proxy.created {
		err = checkError(resp, http.StatusOK, "Save")
	} else {
		err = checkError(resp, http.StatusCreated, "Create")
	}
	if err != nil {
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return err
	}
	proxy.created = true

	return nil
}

// Enable a proxy again after it has been disabled.
func (proxy *Proxy) Enable() error {
	proxy.Enabled = true
	return proxy.Save()
}

// Disable a proxy so that no connections can pass through. This will drop all active connections.
func (proxy *Proxy) Disable() error {
	proxy.Enabled = false
	return proxy.Save()
}

// Delete a proxy complete and close all existing connections through it. All information about
// the proxy such as listen port and active toxics will be deleted as well. If you just wish to
// stop and later enable a proxy, use `Enable()` and `Disable()`.
func (proxy *Proxy) Delete() error {
	httpClient := &http.Client{}
	req, err := http.NewRequest("DELETE", proxy.client.endpoint+"/proxies/"+proxy.Name, nil)

	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	return checkError(resp, http.StatusNoContent, "Delete")
}

// Toxics returns a map of all the active toxics and their attributes.
func (proxy *Proxy) Toxics() (Toxics, error) {
	resp, err := http.Get(proxy.client.endpoint + "/proxies/" + proxy.Name + "/toxics")
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "Toxics")
	if err != nil {
		return nil, err
	}

	toxics := make(Toxics, 0)
	err = json.NewDecoder(resp.Body).Decode(&toxics)
	if err != nil {
		return nil, err
	}

	return toxics, nil
}

// AddToxic adds a toxic to the given stream direction.
// If a name is not specified, it will default to <type>_<stream>.
// If a stream is not specified, it will default to downstream.
// See https://github.com/Shopify/toxiproxy#toxics for a list of all Toxic types.
func (proxy *Proxy) AddToxic(name, typeName, stream string, toxicity float32, attrs Attributes) (*Toxic, error) {
	toxic := Toxic{name, typeName, stream, toxicity, attrs}
	if toxic.Toxicity == -1 {
		toxic.Toxicity = 1 // Just to be consistent with a toxicity of -1 using the default
	}

	request, err := json.Marshal(&toxic)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies/"+proxy.Name+"/toxics", "application/json", bytes.NewReader(request))
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "AddToxic")
	if err != nil {
		return nil, err
	}

	result := &Toxic{}
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UpdateToxic sets the parameters for an existing toxic with the given name.
// If toxicity is set to -1, the current value will be used.
func (proxy *Proxy) UpdateToxic(name string, toxicity float32, attrs Attributes) (*Toxic, error) {
	toxic := map[string]interface{}{
		"attributes": attrs,
	}
	if toxicity != -1 {
		toxic["toxicity"] = toxicity
	}
	request, err := json.Marshal(&toxic)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies/"+proxy.Name+"/toxics/"+name, "application/json", bytes.NewReader(request))
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "UpdateToxic")
	if err != nil {
		return nil, err
	}

	result := &Toxic{}
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// RemoveToxic renives the toxic with the given name.
func (proxy *Proxy) RemoveToxic(name string) error {
	httpClient := &http.Client{}
	req, err := http.NewRequest("DELETE", proxy.client.endpoint+"/proxies/"+proxy.Name+"/toxics/"+name, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	return checkError(resp, http.StatusNoContent, "RemoveToxic")
}

// ResetState resets the state of all proxies and toxics in Toxiproxy.
func (client *Client) ResetState() error {
	resp, err := http.Post(client.endpoint+"/reset", "text/plain", bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}

	return checkError(resp, http.StatusNoContent, "ResetState")
}

type ApiError struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
}

func (err *ApiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", err.Status, err.Title)
}

func checkError(resp *http.Response, expectedCode int, caller string) error {
	if resp.StatusCode != expectedCode {
		apiError := new(ApiError)
		err := json.NewDecoder(resp.Body).Decode(apiError)
		if err != nil {
			apiError.Title = fmt.Sprintf("Unexpected response code, expected %d", expectedCode)
			apiError.Status = resp.StatusCode
		}
		return fmt.Errorf("%s: %v", caller, apiError)
	}
	return nil
}
