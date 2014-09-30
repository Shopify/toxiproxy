package main

import (
	"bytes"
	"io/ioutil"
	"net"
	"testing"
)

func NewTestProxy(name, upstream string) *Proxy {
	proxy := NewProxy()

	proxy.Name = name
	proxy.Listen = "localhost:20000"
	proxy.Upstream = upstream

	return proxy
}

func WithTCPServer(t *testing.T, f func(string, chan []byte)) {
	ln, err := net.Listen("tcp", "localhost:20002")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}

	defer ln.Close()

	response := make(chan []byte, 1)
	quit := make(chan bool, 1)
	done := make(chan bool)

	go func() {
		defer close(done)
		src, err := ln.Accept()
		if err != nil {
			select {
			case <-quit:
			default:
				t.Fatal("Failed to accept client")
			}
			return
		}

		err = ln.Close()
		if err != nil {
			select {
			case <-quit:
			default:
				t.Fatal("Failed to close listener")
			}
			return
		}

		val, err := ioutil.ReadAll(src)
		if err != nil {
			t.Fatal("Failed to read from client")
		}

		response <- val
	}()

	f(ln.Addr().String(), response)

	quit <- true
	ln.Close()
	<-done

	close(response)
}

func TestSimpleServer(t *testing.T) {
	WithTCPServer(t, func(addr string, response chan []byte) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}

		msg := []byte("hello world")

		_, err = conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to TCP server", err)
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read bytes from client")
		}
	})
}

func WithTCPProxy(t *testing.T, f func(proxy net.Conn, response chan []byte, proxyServer *Proxy)) {
	WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()

		conn, err := net.Dial("tcp", "localhost:20000")
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}

		f(conn, response, proxy)

		proxy.Stop()
	})
}

func TestProxySimpleMessage(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		msg := []byte("hello world")

		_, err := conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to TCP server", err)
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read correct bytes from client", resp)
		}
	})
}

func TestProxyTwoPartMessage(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		msg1 := []byte("hello world")
		msg2 := []byte("hello world")

		_, err := conn.Write(msg1)
		if err != nil {
			t.Error("Failed writing to TCP server", err)
		}

		_, err = conn.Write(msg2)
		if err != nil {
			t.Error("Failed writing to TCP server", err)
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}

		msg1 = append(msg1, msg2...)

		resp := <-response
		if !bytes.Equal(resp, msg1) {
			t.Error("Server didn't read correct bytes from client", resp)
		}
	})
}

func TestClosingProxyMultipleTimes(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		proxy.Stop()
		proxy.Stop()
		proxy.Stop()
	})
}

func TestStartTwoProxiesOnSameAddress(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		proxy2 := NewTestProxy("proxy_2", "localhost:3306")
		if err := proxy2.Start(); err == nil {
			t.Fatal("Expected an err back from start")
		}
	})
}
