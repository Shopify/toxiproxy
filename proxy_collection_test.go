package main

import (
	"bytes"
	"net"
	"testing"
)

func TestAddProxyToCollection(t *testing.T) {
	collection := NewProxyCollection()
	proxy := NewTestProxy("test", "localhost:20000")

	if _, exists := collection.Proxies()[proxy.Name]; exists {
		t.Error("Expected proxies to be empty")
	}

	err := collection.Add(proxy)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	if _, exists := collection.Proxies()[proxy.Name]; !exists {
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

func TestAddTwoProxiesWithSameProxyListen(t *testing.T) {
	collection := NewProxyCollection()
	proxy1 := NewTestProxy("test", "localhost:20000")

	err := collection.Add(proxy1)
	if err != nil {
		t.Error("Expected to be able to add first proxy to collection")
	}

	proxy2 := NewTestProxy("test2", "localhost:20000")
	proxy2.Listen = proxy1.Listen

	err = collection.Add(proxy2)
	if err == nil {
		t.Error("Expected to not be able to add proxy with same listen")
	}
}

func TestRemoveNonExistantProxy(t *testing.T) {
	collection := NewProxyCollection()

	if err := collection.Remove(".*"); err == nil {
		t.Fatal("Expected an error of no proxies")
	}
}

func TestAddAndRemoveProxyFromCollection(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		collection := NewProxyCollection()

		if _, exists := collection.Proxies()[proxy.Name]; exists {
			t.Error("Expected proxies to be empty")
		}

		err := collection.Add(proxy)
		if err != nil {
			t.Error("Expected to be able to add first proxy to collection")
		}

		if _, exists := collection.Proxies()[proxy.Name]; !exists {
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

		if _, exists := collection.Proxies()[proxy.Name]; exists {
			t.Error("Expected proxies to be empty")
		}
	})
}

func TestAddAndRemoveProxyFromCollectionWithRegex(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		collection := NewProxyCollection()

		if _, exists := collection.Proxies()[proxy.Name]; exists {
			t.Error("Expected proxies to be empty")
		}

		err := collection.Add(proxy)
		if err != nil {
			t.Error("Expected to be able to add first proxy to collection")
		}

		if _, exists := collection.Proxies()[proxy.Name]; !exists {
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

		err = collection.Remove(".*")
		if err != nil {
			t.Error("Expected to remove proxy from collection")
		}

		if _, exists := collection.Proxies()[proxy.Name]; exists {
			t.Error("Expected proxies to be empty")
		}
	})
}
