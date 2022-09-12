// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x
package toxiproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Proxy struct {
	Name     string `json:"name"`     // The name of the proxy
	Listen   string `json:"listen"`   // The address the proxy listens on
	Upstream string `json:"upstream"` // The upstream address to proxy to
	Enabled  bool   `json:"enabled"`  // Whether the proxy is enabled

	ActiveToxics Toxics `json:"toxics"` // The toxics active on this proxy

	client  *Client
	created bool // True if this proxy exists on the server
}

// Save saves changes to a proxy such as its enabled status or upstream port.
func (proxy *Proxy) Save() error {
	request, err := json.Marshal(proxy)
	if err != nil {
		return err
	}

	path := proxy.client.endpoint + "/proxies"
	contenttype := "application/json"
	if proxy.created {
		path += "/" + proxy.Name
		contenttype = "text/plain"
	}

	resp, err := http.Post(path, contenttype, bytes.NewReader(request))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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

	resp, err := http.Post(
		proxy.client.endpoint+"/proxies/"+proxy.Name+"/toxics",
		"application/json",
		bytes.NewReader(request),
	)
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

	resp, err := http.Post(
		proxy.client.endpoint+"/proxies/"+proxy.Name+"/toxics/"+name,
		"application/json",
		bytes.NewReader(request),
	)
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
	req, err := http.NewRequest(
		"DELETE",
		proxy.client.endpoint+"/proxies/"+proxy.Name+"/toxics/"+name,
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	return checkError(resp, http.StatusNoContent, "RemoveToxic")
}
