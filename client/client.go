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
}

type ProxyWithToxics struct {
	Proxy
	ToxicsUpstream   map[string]interface{} `json:"upstream_toxics"`
	ToxicsDownstream map[string]interface{} `json:"downstream_toxics"`
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

	return proxies, nil
}

func (client *Client) CreateProxy(proxy *Proxy) (*Proxy, error) {
	request, err := json.Marshal(proxy)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(client.endpoint+"/proxies", "application/json", bytes.NewReader(request))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		// TODO  better error
		fmt.Printf("OMG status=%d", resp.StatusCode)
		return nil, errors.New("unable to create proxy")
	}

	proxy = new(Proxy)
	err = json.NewDecoder(resp.Body).Decode(&proxy)
	if err != nil {
		return nil, err
	}

	return proxy, nil
}

func (client *Client) InspectProxy(name string) (*ProxyWithToxics, error) {
	// TODO url encode
	resp, err := http.Get(client.endpoint + "/proxies/" + name)
	if err != nil {
		return nil, err
	}

	proxy := &ProxyWithToxics{}
	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return nil, err
	}

	return proxy, nil
}

// TODO should update return toxics?
func (client *Client) UpdateProxy(newProxy *Proxy) (*ProxyWithToxics, error) {
	request, err := json.Marshal(newProxy)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(client.endpoint+"/proxies/"+newProxy.Name, "application/json", bytes.NewReader(request))
	if err != nil {
		return nil, err
	}

	proxy := &ProxyWithToxics{}
	err = json.NewDecoder(resp.Body).Decode(proxy)
	if err != nil {
		return nil, err
	}

	return proxy, nil
}

func (client *Client) DeleteProxy(name string) error {
	httpClient := &http.Client{}
	req, err := http.NewRequest("DELETE", client.endpoint+"/proxies/"+name, nil)

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

func (client *Client) Toxics() (map[string]*ProxyWithToxics, error) {
	resp, err := http.Get(client.endpoint + "/toxics")
	if err != nil {
		return nil, err
	}

	proxies := make(map[string]*ProxyWithToxics)
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
