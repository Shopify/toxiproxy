package toxiproxy_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/Shopify/toxiproxy/v2"
)

func TestAddProxyToCollection(t *testing.T) {
	collection := toxiproxy.NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	if _, err := collection.Get(proxy.Name); err == nil {
		t.Error("Expected proxies to be empty")
	}

	err := collection.Add(proxy, false)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	if _, err := collection.Get(proxy.Name); err != nil {
		t.Error("Expected proxy to be added to map")
	}
}

func TestAddTwoProxiesToCollection(t *testing.T) {
	collection := toxiproxy.NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	err := collection.Add(proxy, false)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	err = collection.Add(proxy, false)
	if err == nil {
		t.Error("Expected to not be able to add proxy with same name")
	}
}

func TestAddOrReplaceProxyToCollectionWithRandomPort(t *testing.T) {
	collection := toxiproxy.NewProxyCollection()
	proxy1 := NewTestProxy("test", "localhost:20000")
	proxy2 := NewTestProxy("test", "localhost:0")

	err := collection.AddOrReplace(proxy1, false)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	err = collection.AddOrReplace(proxy2, false)
	if err != nil {
		t.Error("Expected to be able to add second proxy to collection")
	}

	proxyByName, err := collection.Get("test")
	if err != nil {
		t.Error("Expected to find proxy in collection")
	}
	if proxyByName.Listen != proxy1.Listen {
		t.Error("Expected original `.Listen` to be unchanged")
	}
}

func TestListProxies(t *testing.T) {
	collection := toxiproxy.NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	err := collection.Add(proxy, false)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	proxies := collection.Proxies()
	proxy, ok := proxies[proxy.Name]
	if !ok {
		t.Error("Expected to be able to see existing proxy")
	} else if proxy.Enabled {
		t.Error("Expected proxy not to be running")
	}
}

func TestAddProxyAndStart(t *testing.T) {
	collection := toxiproxy.NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	err := collection.Add(proxy, true)
	if err != nil {
		t.Error("Expected to be able to add proxy to collection:", err)
	}

	proxies := collection.Proxies()
	proxy, ok := proxies[proxy.Name]
	if !ok {
		t.Error("Expected to be able to see existing proxy")
	} else if !proxy.Enabled {
		t.Error("Expected proxy to be running")
	}
}

func TestAddAndRemoveProxyFromCollection(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		collection := toxiproxy.NewProxyCollection()

		if _, err := collection.Get(proxy.Name); err == nil {
			t.Error("Expected proxies to be empty")
		}

		err := collection.Add(proxy, false)
		if err != nil {
			t.Error("Expected to be able to add first proxy to collection")
		}

		if _, err := collection.Get(proxy.Name); err != nil {
			t.Error("Expected proxy to be added to map")
		}

		msg := []byte("go away")

		_, err = conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to socket to shut down server")
		}

		conn.Close()

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read bytes from client")
		}

		err = collection.Remove(proxy.Name)
		if err != nil {
			t.Error("Expected to remove proxy from collection")
		}

		if _, err := collection.Get(proxy.Name); err == nil {
			t.Error("Expected proxies to be empty")
		}
	})
}
