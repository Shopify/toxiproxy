// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
package toxiproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client holds information about where to connect to Toxiproxy.
type Client struct {
	endpoint string
}

type Toxic map[string]interface{}
type Toxics map[string]Toxic

// Proxy represents a Proxy.
type Proxy struct {
	Name     string `json:"name"`     // The name of the proxy
	Listen   string `json:"listen"`   // The address the proxy listens on
	Upstream string `json:"upstream"` // The upstream address to proxy to
	Enabled  bool   `json:"enabled"`  // Whether the proxy is enabled

	ToxicsUpstream   Toxics `json:"upstream_toxics"`   // Toxics in the upstream direction
	ToxicsDownstream Toxics `json:"downstream_toxics"` // Toxics in the downstream direction

	client *Client
}

// NewClient creates a new client which provides the base of all communication
// with Toxiproxy. Endpoint is the address to the proxy (e.g. localhost:8474 if
// not overriden)
func NewClient(endpoint string) *Client {
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
	}

	return proxies, nil
}

// NewProxy instantiates a new proxy instance. Note Create() must be called on
// it to create it. The Enabled field must be set to true, otherwise the Proxy
// will not be enabled when created.
func (client *Client) NewProxy(proxy *Proxy) *Proxy {
	if proxy == nil {
		proxy = &Proxy{}
	}

	proxy.client = client
	return proxy
}

// Create creates a new proxy.
func (proxy *Proxy) Create() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies", "application/json", bytes.NewReader(request))
	if err != nil {
		return err
	}

	err = checkError(resp, http.StatusCreated, "Create")
	if err != nil {
		return err
	}

	proxy = new(Proxy)
	err = json.NewDecoder(resp.Body).Decode(&proxy)
	if err != nil {
		return err
	}

	return nil
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

	proxy := client.NewProxy(nil)
	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return nil, err
	}

	return proxy, nil
}

// Save saves changes to a proxy such as its enabled status.
func (proxy *Proxy) Save() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies/"+proxy.Name, "application/json", bytes.NewReader(request))
	if err != nil {
		return err
	}

	err = checkError(resp, http.StatusOK, "Save")
	if err != nil {
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return err
	}

	return nil
}

// Delete a proxy which will cause it to stop listening and delete all
// information associated with it. If you just wish to stop and later enable a
// proxy, set the `Enabled` field to `false` and call `Save()`.
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

// Toxics returns a map of all the toxics and their attributes for a direction.
func (proxy *Proxy) Toxics(direction string) (Toxics, error) {
	resp, err := http.Get(proxy.client.endpoint + "/proxies/" + proxy.Name + "/" + direction + "/toxics")
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "Toxics")
	if err != nil {
		return nil, err
	}

	toxics := make(Toxics)
	err = json.NewDecoder(resp.Body).Decode(&toxics)
	if err != nil {
		return nil, err
	}

	return toxics, nil
}

// SetToxic sets the parameters for a toxic with a given name in the direction.
// See https://github.com/Shopify/toxiproxy#toxics for a list of all Toxics.
func (proxy *Proxy) SetToxic(name string, direction string, toxic Toxic) (Toxic, error) {
	request, err := json.Marshal(toxic)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies/"+proxy.Name+"/"+direction+"/toxics/"+name, "application/json", bytes.NewReader(request))
	if err != nil {
		return nil, err
	}

	err = checkError(resp, http.StatusOK, "SetToxic")
	if err != nil {
		return nil, err
	}

	toxics := make(Toxic)
	err = json.NewDecoder(resp.Body).Decode(&toxics)
	if err != nil {
		return nil, err
	}

	return toxics, nil
}

// ResetState resets the state of all proxies and toxics in Toxiproxy.
func (client *Client) ResetState() error {
	resp, err := http.Get(client.endpoint + "/reset")
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
