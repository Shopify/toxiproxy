package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

var testServer *server

type ProxyWithToxics struct {
	Proxy
	ToxicsUpstream   map[string]interface{} `json:"upstream_toxics"`
	ToxicsDownstream map[string]interface{} `json:"downstream_toxics"`
}

func WithServer(t *testing.T, f func(string)) {
	// Make sure only one server is running at a time. Apparently there's no clean
	// way to shut it down between each test run.
	if testServer == nil {
		testServer = NewServer(NewProxyCollection())
		go testServer.Listen("localhost", "8475")

		// Allow server to start. There's no clean way to know when it listens.
		time.Sleep(50 * time.Millisecond)
	}

	f("http://localhost:8475")

	err := testServer.collection.Clear()
	if err != nil {
		t.Error("Failed to clear collection", err)
	}
}

func CreateProxy(t *testing.T, addr string, name string) *http.Response {
	body := `
	{
		"name": "` + name + `",
		"listen": "localhost:3310",
		"upstream": "localhost:20001"
	}`

	resp, err := http.Post(addr+"/proxies", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	return resp
}

func CreateDisabledProxy(t *testing.T, addr string, name string) *http.Response {
	body := `
	{
		"name": "` + name + `",
		"listen": "localhost:3310",
		"upstream": "localhost:20001",
		"enabled": false
	}`

	resp, err := http.Post(addr+"/proxies", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	return resp
}

func ListProxies(t *testing.T, addr string) map[string]*Proxy {
	resp, err := http.Get(addr + "/proxies")
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	proxies := make(map[string]*Proxy)
	err = json.NewDecoder(resp.Body).Decode(&proxies)
	if err != nil {
		t.Fatal("Failed to parse JSON response from index")
	}

	return proxies
}

func ListProxiesWithToxics(t *testing.T, addr string) map[string]*ProxyWithToxics {
	resp, err := http.Get(addr + "/toxics")
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	proxies := make(map[string]*ProxyWithToxics)
	err = json.NewDecoder(resp.Body).Decode(&proxies)
	if err != nil {
		t.Fatal("Failed to parse JSON response from index")
	}

	return proxies
}

func DeleteProxy(t *testing.T, addr, name string) *http.Response {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", addr+"/proxies/"+name, nil)
	if err != nil {
		t.Fatal("Failed to create request", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("Failed to issue request", err)
	}

	return resp
}

func ResetState(t *testing.T, addr string) *http.Response {
	client := &http.Client{}
	req, err := http.NewRequest("GET", addr+"/reset", nil)
	if err != nil {
		t.Fatal("Failed to create request", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("Failed to issue request", err)
	}

	return resp
}

func ListToxics(t *testing.T, addr, proxy, direction string) map[string]interface{} {
	resp, err := http.Get(addr + "/proxies/" + proxy + "/" + direction + "/toxics")
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	toxics := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&toxics)
	if err != nil {
		t.Fatal("Failed to parse JSON response from index")
	}

	return toxics
}

func ShowProxy(t *testing.T, addr, proxy string) ProxyWithToxics {
	resp, err := http.Get(addr + "/proxies/" + proxy)
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	var p ProxyWithToxics
	err = json.NewDecoder(resp.Body).Decode(&p)
	if err != nil {
		t.Fatal("Failed to parse JSON response from index")
	}

	return p
}

func ProxyUpdate(t *testing.T, addr, proxy, data string) ProxyWithToxics {
	resp, err := http.Post(addr+"/proxies/"+proxy, "application/json", strings.NewReader(data))
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	var p ProxyWithToxics
	err = json.NewDecoder(resp.Body).Decode(&p)
	if err != nil {
		t.Fatal("Failed to parse JSON response from index")
	}

	return p
}

func SetToxic(t *testing.T, addr, proxy, direction, name, toxic string) map[string]interface{} {
	resp, err := http.Post(addr+"/proxies/"+proxy+"/"+direction+"/toxics/"+name, "application/json", strings.NewReader(toxic))
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	toxics := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&toxics)
	if err != nil {
		t.Fatal("Failed to parse JSON response from index")
	}

	return toxics
}

func TestIndexWithNoProxies(t *testing.T) {
	WithServer(t, func(addr string) {
		if len(ListProxies(t, addr)) > 0 {
			t.Fatal("Expected no proxies in list")
		}
	})
}

func TestCreateProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}
	})
}

func TestIndexWithProxies(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		proxies := ListProxies(t, addr)
		if len(proxies) == 0 {
			t.Fatal("Expected new proxy in list")
		}
	})
}

func TestIndexWithToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		proxies := ListProxiesWithToxics(t, addr)
		if len(proxies) == 0 {
			t.Fatal("Expected new proxy in list")
		}
		proxy, ok := proxies["mysql_master"]
		if !ok {
			t.Fatal("Expected to see mysql_master proxy in list")
		}
		if proxy.Name != "mysql_master" || proxy.Listen != "127.0.0.1:3310" || proxy.Upstream != "localhost:20001" {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s", proxy.Name, proxy.Listen, proxy.Upstream)
		}
		AssertToxicEnabled(t, proxy.ToxicsUpstream, "latency", false)
		AssertToxicEnabled(t, proxy.ToxicsDownstream, "latency", false)
	})
}

func TestShowProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		proxy := ShowProxy(t, addr, "mysql_master")
		if proxy.Name != "mysql_master" || proxy.Listen != "127.0.0.1:3310" || proxy.Upstream != "localhost:20001" || !proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		AssertToxicEnabled(t, proxy.ToxicsUpstream, "latency", false)
		AssertToxicEnabled(t, proxy.ToxicsDownstream, "latency", false)
	})
}

func TestEnableProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateDisabledProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		proxy := ShowProxy(t, addr, "mysql_master")
		if proxy.Name != "mysql_master" || proxy.Listen != "localhost:3310" || proxy.Upstream != "localhost:20001" || proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		proxy = ProxyUpdate(t, addr, "mysql_master", `{"enabled": true}`)
		if proxy.Name != "mysql_master" || proxy.Listen != "127.0.0.1:3310" || proxy.Upstream != "localhost:20001" || !proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		proxy = ProxyUpdate(t, addr, "mysql_master", `{"enabled": false}`)
		if proxy.Name != "mysql_master" || proxy.Listen != "127.0.0.1:3310" || proxy.Upstream != "localhost:20001" || proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}
	})
}

func TestCreateDisabledProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateDisabledProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		proxy := ShowProxy(t, addr, "mysql_master")
		if proxy.Name != "mysql_master" || proxy.Listen != "localhost:3310" || proxy.Upstream != "localhost:20001" || proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		AssertProxyUp(t, &proxy.Proxy, false)
	})
}

func TestDeleteProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		proxies := ListProxies(t, addr)
		if len(proxies) == 0 {
			t.Fatal("Expected new proxy in list")
		}

		if resp := DeleteProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusNoContent {
			t.Fatal("Unable to delete proxy")
		}

		proxies = ListProxies(t, addr)
		if len(proxies) > 0 {
			t.Fatal("Expected proxy to be deleted from list")
		}
	})
}

func TestCreateProxyTwice(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusConflict {
			t.Fatal("Expected http.StatusConflict Conflict back from API")
		}
	})
}

func TestDeleteNonExistantProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := DeleteProxy(t, addr, "non_existant"); resp.StatusCode != http.StatusNotFound {
			t.Fatal("Expected http.StatusNotFound Not found when deleting non existant proxy")
		}
	})
}

func TestResetState(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateDisabledProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		latency := SetToxic(t, addr, "mysql_master", "downstream", "latency", `{"enabled": true, "latency": 100, "jitter": 10}`)
		if latency["enabled"] != true {
			t.Fatal("Latency toxic did not start up")
		}
		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings")
		}

		if resp := ResetState(t, addr); resp.StatusCode != http.StatusNoContent {
			t.Fatal("Unable to reset state")
		}

		proxies := ListProxies(t, addr)
		proxy, ok := proxies["mysql_master"]
		if !ok {
			t.Fatal("Expected proxy to still exist")
		}
		if !proxy.Enabled {
			t.Fatal("Expected proxy to be enabled")
		}

		toxics := ListToxics(t, addr, "mysql_master", "downstream")
		latency = AssertToxicEnabled(t, toxics, "latency", false)
		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not keep settings on reset")
		}

		AssertProxyUp(t, proxy, true)
	})
}

func TestListToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		toxics := ListToxics(t, addr, "mysql_master", "upstream")
		AssertToxicEnabled(t, toxics, "latency", false)
	})
}

func TestSetToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		latency := SetToxic(t, addr, "mysql_master", "downstream", "latency", `{"enabled": true, "latency": 100, "jitter": 10}`)
		if latency["enabled"] != true {
			t.Fatal("Latency toxic did not start up")
		}
		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings")
		}

		toxics := ListToxics(t, addr, "mysql_master", "downstream")
		AssertToxicEnabled(t, toxics, "latency", true)

		toxics = ListToxics(t, addr, "mysql_master", "upstream")
		AssertToxicEnabled(t, toxics, "latency", false)
	})
}

func TestUpdateToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		latency := SetToxic(t, addr, "mysql_master", "downstream", "latency", `{"enabled": true, "latency": 100, "jitter": 10}`)
		if latency["enabled"] != true {
			t.Fatal("Latency toxic did not start up")
		}
		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings")
		}

		latency = SetToxic(t, addr, "mysql_master", "downstream", "latency", `{"latency": 1000}`)
		if latency["enabled"] != true {
			t.Fatal("Latency toxic did not stay enabled")
		}
		if latency["latency"] != 1000.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not get updated with the correct settings")
		}
	})
}

func TestVersionEndpointReturnsVersion(t *testing.T) {
	WithServer(t, func(addr string) {
		resp, err := http.Get(addr + "/version")
		if err != nil {
			t.Fatal("Failed to get index", err)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal("Unable to read body from response")
		}

		if string(body) != Version {
			t.Fatal("Expected to return Version from /version, got:", string(body))
		}
	})
}

func AssertToxicEnabled(t *testing.T, toxics map[string]interface{}, name string, enabled bool) map[string]interface{} {
	toxic, ok := toxics[name]
	if !ok {
		t.Fatalf("Expected to see %s toxic in list", name)
		return nil
	}
	toxicMap, ok := toxic.(map[string]interface{})
	if !ok {
		t.Fatal("Couldn't read toxic as a %s toxic", name)
		return nil
	}
	if toxicMap["enabled"] != enabled {
		t.Fatal("%s toxic should have had enabled = %v", name, enabled)
		return nil
	}
	return toxicMap
}
