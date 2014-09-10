package main

import (
	"encoding/json"
	"fmt"
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
		go testServer.Listen()

		// Allow server to start. There's no clean way to know when it listens.
		time.Sleep(50 * time.Millisecond)
	}

	f("http://localhost:8474")

	err := testServer.collection.Clear()
	if err != nil {
		t.Error("Failed to clear collection", err)
	}
}

func CreateProxy(t *testing.T, addr string, name string) *http.Response {
	body := fmt.Sprintf(`
	{
		"Name": "%s",
		"Upstream": "localhost:20001"
	}`, name)

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

func DeleteProxy(t *testing.T, addr string, name string) *http.Response {
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

func TestCreateProxyAndRemoveByWildcard(t *testing.T) {
	WithServer(t, func(addr string) {
		if resp := CreateProxy(t, addr, "mysql_master"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Expected http.StatusConflict Conflict back from API")
		}

		if resp := CreateProxy(t, addr, "mysql_slave"); resp.StatusCode != http.StatusCreated {
			t.Fatal("Expected http.StatusConflict Conflict back from API")
		}

		if len(ListProxies(t, addr)) != 2 {
			t.Fatal("Expected to have created two new proxies")
		}

		if resp := DeleteProxy(t, addr, "mysql_.*"); resp.StatusCode != http.StatusNoContent {
			t.Fatal("Expected to delete all mysql proxies with wildcard")
		}

		if len(ListProxies(t, addr)) != 0 {
			t.Fatal("Expected all proxies to be deleted")
		}
	})
}
