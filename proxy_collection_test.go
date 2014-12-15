package main

import (
	"bytes"
	"errors"
	"net"
	"testing"
)

func TestAddProxyToCollection(t *testing.T) {
	collection := NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	if _, err := collection.Get(proxy.Name); err == nil {
		t.Error("Expected proxies to be empty")
	}

	err := collection.Add(proxy)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	if _, err := collection.Get(proxy.Name); err != nil {
		t.Error("Expected proxy to be added to map")
	}
}

func TestAddTwoProxiesToCollection(t *testing.T) {
	collection := NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	err := collection.Add(proxy)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	err = collection.Add(proxy)
	if err == nil {
		t.Error("Expected to not be able to add proxy with same name")
	}
}

func TestListProxiesBlock(t *testing.T) {
	collection := NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	err := collection.Add(proxy)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	testErr := errors.New("Test error returns")
	err = collection.Proxies(
		func(proxies map[string]*Proxy) error {
			if _, ok := proxies[proxy.Name]; !ok {
				t.Error("Expected to be able to see existing proxies inside block")
			}
			return testErr
		},
	)
	if err != testErr {
		t.Error("Expected to see return value from block")
	}
}

func TestAddAndRemoveProxyFromCollection(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		collection := NewProxyCollection()

		if _, err := collection.Get(proxy.Name); err == nil {
			t.Error("Expected proxies to be empty")
		}

		err := collection.Add(proxy)
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
