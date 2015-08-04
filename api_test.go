package main

import (
	"io/ioutil"
	"net/http"
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
		testServer = NewServer()
		go testServer.Listen("localhost", "8475")

		// Allow server to start. There's no clean way to know when it listens.
		time.Sleep(50 * time.Millisecond)
	}

	defer func() {
		err := testServer.collection.Clear()
		if err != nil {
			t.Error("Failed to clear collection", err)
		}
	}()

	f("http://localhost:8475")
}

func TestIndexWithNoProxies(t *testing.T) {
	WithServer(t, func(addr string) {
		client := tclient.NewClient(addr)
		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Failed getting proxies:", err)
		}

		if len(proxies) > 0 {
			t.Fatal("Expected no proxies, got:", proxies)
		}
	})
}

func TestCreateProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}
	})
}

func TestCreateProxyBlankName(t *testing.T) {
	WithServer(t, func(addr string) {
		blankProxy := client.NewProxy(&tclient.Proxy{})
		err := blankProxy.Create()
		if err == nil {
			t.Fatal("Expected error creating proxy, got nil")
		} else if err.Error() != "Create: HTTP 400: missing required field: name" {
			t.Fatal("Expected different error creating proxy:", err)
		}
	})
}

func TestCreateProxyBlankUpstream(t *testing.T) {
	WithServer(t, func(addr string) {
		blankProxy := client.NewProxy(&tclient.Proxy{Name: "test"})
		err := blankProxy.Create()
		if err == nil {
			t.Fatal("Expected error creating proxy, got nil")
		} else if err.Error() != "Create: HTTP 400: missing required field: upstream" {
			t.Fatal("Expected different error creating proxy:", err)
		}
	})
}

func TestIndexWithToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies:", err)
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
		AssertToxicExists(t, proxy.ToxicsUpstream, "latency", "", false)
		AssertToxicExists(t, proxy.ToxicsDownstream, "latency", "", false)
	})
}

func TestGetProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		proxy, err := client.Proxy("mysql_master")
		if err != nil {
			t.Fatal("Unable to retriecve proxy:", err)
		}

		if proxy.Name != "mysql_master" || proxy.Listen != "127.0.0.1:3310" || proxy.Upstream != "localhost:20001" || !proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		AssertToxicExists(t, proxy.ToxicsUpstream, "latency", "", false)
		AssertToxicExists(t, proxy.ToxicsDownstream, "latency", "", false)
	})
}

func TestCreateDisabledProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		disabledProxy := *testProxy
		disabledProxy.Enabled = false

		err := disabledProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		proxy, err := client.Proxy("mysql_master")
		if err != nil {
			t.Fatal("Unable to retriecve proxy:", err)
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
			t.Fatal("Unable to create proxy:", err)
		}

		proxy, err := client.Proxy("mysql_master")
		if err != nil {
			t.Fatal("Unable to retriecve proxy:", err)
		}

		if proxy.Name != "mysql_master" || proxy.Listen != "localhost:3310" || proxy.Upstream != "localhost:20001" || proxy.Enabled {
			t.Fatalf("Unexpected proxy metadata: %s, %s, %s, %v", proxy.Name, proxy.Listen, proxy.Upstream, proxy.Enabled)
		}

		proxy.Enabled = true

		err = proxy.Save()
		if err != nil {
			t.Fatal("Failed to update proxy:", err)
		}

		AssertProxyUp(t, proxy.Listen, true)

		proxy.Enabled = false

		err = proxy.Save()
		if err != nil {
			t.Fatal("Failed to update proxy:", err)
		}

		AssertProxyUp(t, proxy.Listen, false)
	})
}

func TestDeleteProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies:", err)
		}

		if len(proxies) == 0 {
			t.Fatal("Expected new proxy in list")
		}

		AssertProxyUp(t, testProxy.Listen, true)

		err = testProxy.Delete()
		if err != nil {
			t.Fatal("Failed deleting proxy:", err)
		}

		AssertProxyUp(t, testProxy.Listen, false)

		proxies, err = client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies:", err)
		}

		if len(proxies) > 0 {
			t.Fatal("Expected proxy to be deleted from list")
		}
	})
}

func TestCreateProxyPortConflict(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		testProxy2 := *testProxy
		testProxy2.Name = "test"
		err = testProxy2.Create()
		if err == nil {
			t.Fatal("Proxy did not result in conflict.")
		} else if err.Error() != "Create: HTTP 500: listen tcp 127.0.0.1:3310: bind: address already in use" {
			t.Fatal("Incorrect error adding proxy:", err)
		}

		err = testProxy.Delete()
		if err != nil {
			t.Fatal("Unable to delete proxy:", err)
		}
		err = testProxy2.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}
	})
}

func TestCreateProxyNameConflict(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		testProxy2 := *testProxy
		testProxy2.Listen = "localhost:3311"
		err = testProxy2.Create()
		if err == nil {
			t.Fatal("Proxy did not result in conflict.")
		} else if err.Error() != "Create: HTTP 409: proxy already exists" {
			t.Fatal("Incorrect error adding proxy:", err)
		}

		err = testProxy.Delete()
		if err != nil {
			t.Fatal("Unable to delete proxy:", err)
		}
		err = testProxy2.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}
	})
}

func TestDeleteNonExistantProxy(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Delete()
		if err == nil {
			t.Fatal("Proxy did not result in not found.")
		} else if err.Error() != "Delete: HTTP 404: proxy not found" {
			t.Fatal("Incorrect error removing proxy:", err)
		}
	})
}

func TestResetState(t *testing.T) {
	WithServer(t, func(addr string) {
		disabledProxy := *testProxy
		disabledProxy.Enabled = false

		err := disabledProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		latency, err := disabledProxy.AddToxic("", "latency", "downstream", tclient.Toxic{
			"latency": 100,
			"jitter":  10,
		})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings")
		}

		err = client.ResetState()
		if err != nil {
			t.Fatal("unable to reset state:", err)
		}

		proxies, err := client.Proxies()
		if err != nil {
			t.Fatal("Error listing proxies:", err)
		}

		proxy, ok := proxies["mysql_master"]
		if !ok {
			t.Fatal("Expected proxy to still exist")
		}
		if !proxy.Enabled {
			t.Fatal("Expected proxy to be enabled")
		}

		toxics, err := proxy.Toxics("downstream")
		if err != nil {
			t.Fatal("Error requesting toxics:", err)
		}

		AssertToxicExists(t, toxics, "latency", "", false)

		AssertProxyUp(t, proxy.Listen, true)
	})
}

func TestListingToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		toxics, err := testProxy.Toxics("upstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}

		AssertToxicExists(t, toxics, "latency", "", false)
	})
}

func TestAddToxic(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		latency, err := testProxy.AddToxic("foobar", "latency", "downstream", tclient.Toxic{
			"latency": 100,
			"jitter":  10,
		})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings")
		}

		toxics, err := testProxy.Toxics("downstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "foobar", "latency", true)

		toxics, err = testProxy.Toxics("upstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "foobar", "", false)
	})
}

func TestAddMultipleToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		_, err = testProxy.AddToxic("latency1", "latency", "downstream", tclient.Toxic{})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		_, err = testProxy.AddToxic("latency2", "latency", "downstream", tclient.Toxic{})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		toxics, err := testProxy.Toxics("downstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "latency1", "latency", true)
		AssertToxicExists(t, toxics, "latency2", "latency", true)

		toxics, err = testProxy.Toxics("upstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "latency1", "", false)
		AssertToxicExists(t, toxics, "latency2", "", false)
	})
}

func TestAddConflictingToxic(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		_, err = testProxy.AddToxic("foobar", "latency", "downstream", tclient.Toxic{})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		_, err = testProxy.AddToxic("foobar", "slow_close", "downstream", tclient.Toxic{})
		if err == nil {
			t.Fatal("Toxic did not result in conflict.")
		} else if err.Error() != "AddToxic: HTTP 409: toxic already exists" {
			t.Fatal("Incorrect error setting toxic:", err)
		}

		toxics, err := testProxy.Toxics("downstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "foobar", "latency", true)

		toxics, err = testProxy.Toxics("upstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "foobar", "", false)
	})
}

func TestUpdateToxics(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		latency, err := testProxy.AddToxic("", "latency", "downstream", tclient.Toxic{
			"latency": 100,
			"jitter":  10,
		})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		if latency["latency"] != 100.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not start up with correct settings:", latency)
		}

		latency, err = testProxy.UpdateToxic("latency", "downstream", tclient.Toxic{
			"latency": 1000,
		})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		if latency["latency"] != 1000.0 || latency["jitter"] != 10.0 {
			t.Fatal("Latency toxic did not get updated with the correct settings")
		}
	})
}

func TestRemoveToxic(t *testing.T) {
	WithServer(t, func(addr string) {
		err := testProxy.Create()
		if err != nil {
			t.Fatal("Unable to create proxy:", err)
		}

		_, err = testProxy.AddToxic("", "latency", "downstream", tclient.Toxic{})
		if err != nil {
			t.Fatal("Error setting toxic:", err)
		}

		toxics, err := testProxy.Toxics("downstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "latency", "latency", true)

		err = testProxy.RemoveToxic("latency", "downstream")
		if err != nil {
			t.Fatal("Error removing toxic:", err)
		}

		toxics, err = testProxy.Toxics("downstream")
		if err != nil {
			t.Fatal("Error returning toxics:", err)
		}
		AssertToxicExists(t, toxics, "latency", "", false)
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

func AssertToxicExists(t *testing.T, toxics tclient.Toxics, name, typeName string, exists bool) tclient.Toxic {
	toxic, ok := toxics[name]
	var actualType string
	if ok {
		actualType = toxic["type"].(string)
	}
	if ok != exists {
		if exists {
			t.Fatalf("Expected to see %s toxic in list", name)
		} else {
			t.Fatalf("Expected %s toxic to be missing from list, found type %s", name, actualType)
		}
		return toxic
	}
	if ok && actualType != typeName {
		t.Fatalf("Expected %s to be of type %s, found %s", name, typeName, actualType)
	}
	return toxic
}
