// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x
package toxiproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Proxy struct {
	Name     string `json:"name"`     // The name of the proxy
	Listen   string `json:"listen"`   // The address the proxy listens on
	Upstream string `json:"upstream"` // The upstream address to proxy to
	Enabled  bool   `json:"enabled"`  // Whether the proxy is enabled

	// The toxics active on this proxy. Note: you cannot set this
	// when passing Proxy into Populate()
	ActiveToxics Toxics `json:"toxics"`

	client  *Client
	created bool // True if this proxy exists on the server
}

// Save saves changes to a proxy such as its enabled status or upstream port.
func (proxy *Proxy) Save() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}
	data := bytes.NewReader(request)

	var resp []byte
	if proxy.created {
		// TODO: Release PATCH only for v3.0
		// resp, err = proxy.client.patch("/proxies/"+proxy.Name, data)
		resp, err = proxy.client.post("/proxies/"+proxy.Name, data)
	} else {
		resp, err = proxy.client.post("/proxies", data)
	}
	if err != nil {
		return err
	}

	err = json.Unmarshal(resp, proxy)
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
	err := proxy.client.delete("/proxies/" + proxy.Name)
	if err != nil {
		return fmt.Errorf("Delete: %w", err)
	}
	return nil
}

// Toxics returns a map of all the active toxics and their attributes.
func (proxy *Proxy) Toxics() (Toxics, error) {
	resp, err := proxy.client.get("/proxies/" + proxy.Name + "/toxics")
	if err != nil {
		return nil, err
	}

	toxics := make(Toxics, 0)
	err = json.Unmarshal(resp, &toxics)
	if err != nil {
		return nil, err
	}

	return toxics, nil
}

// AddToxic adds a toxic to the given stream direction.
// If a name is not specified, it will default to <type>_<stream>.
// If a stream is not specified, it will default to downstream.
// See https://github.com/Shopify/toxiproxy#toxics for a list of all Toxic types.
func (proxy *Proxy) AddToxic(
	name, typeName, stream string,
	toxicity float32,
	attrs Attributes,
) (*Toxic, error) {
	toxic := Toxic{name, typeName, stream, toxicity, attrs}
	if toxic.Toxicity == -1 {
		toxic.Toxicity = 1 // Just to be consistent with a toxicity of -1 using the default
	}

	request, err := json.Marshal(&toxic)
	if err != nil {
		return nil, err
	}

	resp, err := proxy.client.post(
		"/proxies/"+proxy.Name+"/toxics",
		bytes.NewReader(request),
	)
	if err != nil {
		return nil, fmt.Errorf("AddToxic: %w", err)
	}

	result := &Toxic{}
	err = json.Unmarshal(resp, result)
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

	resp, err := proxy.client.patch(
		"/proxies/"+proxy.Name+"/toxics/"+name,
		bytes.NewReader(request),
	)
	if err != nil {
		return nil, err
	}

	result := &Toxic{}
	err = json.Unmarshal(resp, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// RemoveToxic renives the toxic with the given name.
func (proxy *Proxy) RemoveToxic(name string) error {
	return proxy.client.delete("/proxies/" + proxy.Name + "/toxics/" + name)
}
