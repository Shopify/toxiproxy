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

	response := make(chan []byte)

	go func() {
		src, err := ln.Accept()
		if err != nil {
			t.Fatal("Failed to accept client")
		}

		err = ln.Close()
		if err != nil {
			t.Fatal("Failed to close TCP server")
		}

		val, err := ioutil.ReadAll(src)
		if err != nil {
			t.Fatal("Failed to read from client")
		}

		response <- val
	}()

	f(ln.Addr().String(), response)
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
