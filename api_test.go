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
		"Name": "` + name + `",
		"Listen": "localhost:3310",
		"Upstream": "localhost:20001"
	}`

	resp, err := http.Post(addr+"/proxies", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	return resp
}

func ListProxies(t *testing.T, addr string) map[string]Proxy {
	resp, err := http.Get(addr + "/proxies")
	if err != nil {
		t.Fatal("Failed to get index", err)
	}

	proxies := make(map[string]Proxy)
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

func SetToxic(t *testing.T, addr, proxy, direction, name string, toxic string) map[string]interface{} {
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
			t.Error("Expected no proxies in list")
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
			t.Error("Expected new proxy in list")
		}
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
			t.Error("Expected proxy to be deleted from list")
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

func TestListToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Unable to create proxy")
		}

		toxics := ListToxics(t, addr, "mysql_master", "upstream")
		toxic, ok := toxics["latency"]
		if !ok {
			t.Fatal("Expected to see latency toxic in list")
		}
		latency, ok := toxic.(map[string]interface{})
		if !ok {
			t.Fatal("Couldn't read toxic as a latency toxic")
		}
		if latency["enabled"] != false {
			t.Fatal("Latency toxic did not start up as disabled")
		}
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
		toxic, ok := toxics["latency"]
		if !ok {
			t.Fatal("Expected to see latency toxic in list")
		}
		latency, ok = toxic.(map[string]interface{})
		if !ok {
			t.Fatal("Couldn't read toxic as a latency toxic")
		}
		if latency["enabled"] != true {
			t.Fatal("Latency toxic did not stay enabled")
		}

		toxics = ListToxics(t, addr, "mysql_master", "upstream")
		toxic, ok = toxics["latency"]
		if !ok {
			t.Fatal("Expected to see latency toxic in list")
		}
		latency, ok = toxic.(map[string]interface{})
		if !ok {
			t.Fatal("Couldn't read toxic as a latency toxic")
		}
		if latency["enabled"] != false {
			t.Fatal("Upstream toxic should not have been enabled")
		}
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
