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
	"net/http"
	"strings"
	"time"
)

// Client holds information about where to connect to Toxiproxy.
type Client struct {
	UserAgent string
	endpoint  string
	http      *http.Client
}

// NewClient creates a new client which provides the base of all communication
// with Toxiproxy. Endpoint is the address to the proxy (e.g. localhost:8474 if
// not overridden).
func NewClient(endpoint string) *Client {
	if !strings.HasPrefix(endpoint, "https://") &&
		!strings.HasPrefix(endpoint, "http://") {
		endpoint = "http://" + endpoint
	}

	http := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &Client{
		UserAgent: "toxiproxy-cli",
		endpoint:  endpoint,
		http:      http,
	}
}

// Version returns a Toxiproxy running version.
func (client *Client) Version() ([]byte, error) {
	return client.get("/version")
}

// Proxies returns a map with all the proxies and their toxics.
func (client *Client) Proxies() (map[string]*Proxy, error) {
	resp, err := client.get("/proxies")
	if err != nil {
		return nil, err
	}

	proxies := make(map[string]*Proxy)
	err = json.Unmarshal(resp, &proxies)
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
		return nil, fmt.Errorf("create: %w", err)
	}

	return proxy, nil
}

// Proxy returns a proxy by name.
func (client *Client) Proxy(name string) (*Proxy, error) {
	resp, err := client.get("/proxies/" + name)
	if err != nil {
		return nil, err
	}

	proxy := new(Proxy)
	err = json.Unmarshal(resp, &proxy)
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

	resp, err := client.post("/populate", bytes.NewReader(request))
	if err != nil {
		return nil, fmt.Errorf("Populate: %w", err)
	}

	err = json.Unmarshal(resp, &proxies)
	if err != nil {
		return nil, err
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
	_, err := client.post("/reset", bytes.NewReader([]byte{}))
	return err
}

func (c *Client) get(path string) ([]byte, error) {
	return c.send("GET", path, nil)
}

func (c *Client) post(path string, body io.Reader) ([]byte, error) {
	return c.send("POST", path, body)
}

func (c *Client) patch(path string, body io.Reader) ([]byte, error) {
	return c.send("PATCH", path, body)
}

func (c *Client) delete(path string) error {
	_, err := c.send("DELETE", path, nil)
	return err
}

func (c *Client) send(verb, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(verb, c.endpoint+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fail to request: %w", err)
	}

	err = c.validateResponse(resp)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) validateResponse(resp *http.Response) error {
	if resp.StatusCode < 300 && resp.StatusCode >= 200 {
		return nil
	}

	apiError := new(ApiError)
	err := json.NewDecoder(resp.Body).Decode(&apiError)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if err != nil {
		apiError.Message = fmt.Sprintf(
			"Unexpected response code %d",
			resp.StatusCode,
		)
		apiError.Status = resp.StatusCode
	}
	return apiError
}
