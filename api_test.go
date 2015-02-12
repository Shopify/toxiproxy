package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	tclient "github.com/Shopify/toxiproxy/client"
)

var testServer *server

var client = tclient.NewClient("http://127.0.0.1:8475")
var testProxy = client.NewProxy(&tclient.Proxy{
	Name:     "mysql_master",
	Listen:   "localhost:3310",
	Upstream: "localhost:20001",
	Enabled:  true,
})

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
		client := tclient.NewClient(addr)
		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Failed getting proxies: ", err)
		}

		if len(proxies) > 0 {
			t.Fatal("Expected no proxies, got: ", proxies)
		}
	})
}

func TestCreateProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy: ", err)
		}
	})
}

func TestIndexWithProxies(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy")
		}

		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Failed listing proxies: ", err)
		}

		if len(proxies) == 0 {
			t.Fatal("Expected new proxy in list")
		}
	})
}

func TestIndexWithToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy")
		}

		proxies, err := client.Toxics()
		if err != nil {
			t.Fatal("Error listing toxics: ", err)
		}

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

func TestGetProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy")
		}

		proxy, err := client.Proxy("mysql_master")
		if err != nil {
			t.Fatal("Unable to retriecve proxy: ", err)
		}

		if proxy.Name != "mysql_master" || proxy.Listen != "127.0.0.1:3310" || proxy.Upstream != "localhost:20001" || !proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		AssertToxicEnabled(t, proxy.ToxicsUpstream, "latency", false)
		AssertToxicEnabled(t, proxy.ToxicsDownstream, "latency", false)
	})
}

func TestCreateDisabledProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		disabledProxy := *testProxy
		disabledProxy.Enabled = false

		err := disabledProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy: ", err)
		}

		proxy, err := client.Proxy("mysql_master")
		if err != nil {
			t.Fatal("Unable to retriecve proxy: ", err)
		}

		if proxy.Name != "mysql_master" || proxy.Listen != "localhost:3310" || proxy.Upstream != "localhost:20001" || proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		AssertProxyUp(t, proxy.Listen, false)
	})
}

func TestCreateDisabledProxyAndEnable(t *testing.T) {
	WithServer(t, func(addr string) {
		disabledProxy := *testProxy
		disabledProxy.Enabled = false

		err := disabledProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy: ", err)
		}

		proxy, err := client.Proxy("mysql_master")
		if err != nil {
			t.Fatal("Unable to retriecve proxy: ", err)
		}

		if proxy.Name != "mysql_master" || proxy.Listen != "localhost:3310" || proxy.Upstream != "localhost:20001" || proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		proxy.Enabled = true

		err = proxy.Save()
		if err != nil {
			t.Fatal("Failed to update proxy: ", err)
		}

		AssertProxyUp(t, proxy.Listen, true)

		proxy.Enabled = false

		err = proxy.Save()
		if err != nil {
			t.Fatal("Failed to update proxy: ", err)
		}

		AssertProxyUp(t, proxy.Listen, false)
	})
}

func TestDeleteProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy: ", err)
		}

		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies: ", err)
		}

		if len(proxies) == 0 {
			t.Fatal("Expected new proxy in list")
		}

		AssertProxyUp(t, testProxy.Listen, true)

		err = testProxy.Delete()
		if err != nil {
			t.Fatal("Failed deleting proxy: ", err)
		}

		AssertProxyUp(t, testProxy.Listen, false)

		proxies, err = client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies: ", err)
		}

		if len(proxies) > 0 {
			t.Fatal("Expected proxy to be deleted from list")
		}
	})
}

func TestCreateProxyTwice(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy")
		}

		err = testProxy.Create()
		if err == nil {
			t.Fatal("Expected error when creating same proxy twice")
		}
	})
}

func TestDeleteNonExistantProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Delete()
		if err == nil {
			t.Fatal("Expected error when deleting proxy that doesn't exist")
		}
	})
}

func TestResetState(t *testing.T) {
	WithServer(t, func(addr string) {
		disabledProxy := *testProxy
		disabledProxy.Enabled = false

		err := disabledProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy: ", err)
		}

		latency := SetToxic(t, addr, "mysql_master", "downstream", "latency", `{"enabled": true, "latency": 100, "jitter": 10}`)
		if latency["enabled"] != true {
			t.Fatal("Latency toxic did not start up")
		}
		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings")
		}

		err = client.ResetState()
		if err != nil {
			t.Fatal("unable to reset state: ", err)
		}

		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies: ", err)
		}

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

		AssertProxyUp(t, proxy.Listen, true)
	})
}

func TestListToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy")
		}

		toxics := ListToxics(t, addr, "mysql_master", "upstream")
		AssertToxicEnabled(t, toxics, "latency", false)
	})
}

func TestSetToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
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
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy: ", err)
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
