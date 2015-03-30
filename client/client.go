package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Client struct {
	endpoint string
}

type Proxy struct {
	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`

	ToxicsUpstream   map[string]interface{} `json:"upstream_toxics"`
	ToxicsDownstream map[string]interface{} `json:"downstream_toxics"`

	client *Client
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint}
}

func (client *Client) Proxies() (map[string]*Proxy, error) {
	resp, err := http.Get(client.endpoint + "/proxies")
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

func (client *Client) NewProxy(proxy *Proxy) *Proxy {
	if proxy == nil {
		proxy = &Proxy{}
	}

	proxy.client = client
	return proxy
}

func (proxy *Proxy) Create() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies", "application/json", bytes.NewReader(request))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		// TODO  better error
		return fmt.Errorf("omg error code %d", resp.StatusCode)
	}

	proxy = new(Proxy)
	err = json.NewDecoder(resp.Body).Decode(&proxy)
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) Proxy(name string) (*Proxy, error) {
	// TODO url encode
	resp, err := http.Get(client.endpoint + "/proxies/" + name)
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

func (proxy *Proxy) Save() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}

	resp, err := http.Post(proxy.client.endpoint+"/proxies/"+proxy.Name, "application/json", bytes.NewReader(request))
	if err != nil {
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return err
	}

	return nil
}

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

	if resp.StatusCode != http.StatusNoContent {
		// TODO better error
		return errors.New("Status code bad")
	}

	return nil
}

func (proxy *Proxy) Toxics(direction string) (map[string]interface{}, error) {
	resp, err := http.Get(proxy.client.endpoint + "/proxies/" + proxy.Name + "/" + direction + "/toxics")
	if err != nil {
		return nil, err
	}

	toxics := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&toxics)
	if err != nil {
		return nil, err
	}

	return toxics, nil
}

func (client *Client) Toxics() (map[string]*Proxy, error) {
	resp, err := http.Get(client.endpoint + "/toxics")
	if err != nil {
		return nil, err
	}

	proxies := make(map[string]*Proxy)
	err = json.NewDecoder(resp.Body).Decode(&proxies)
	if err != nil {
		return nil, err
	}

	return proxies, nil
}

func (client *Client) ResetState() error {
	resp, err := http.Get(client.endpoint + "/reset")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		// TODO better error
		return errors.New("unable to reset")
	}

	return nil
}
