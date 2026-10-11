package toxiproxy_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/testhelper"
)

func TestProxySimpleMessage(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
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

func TestProxyToDownUpstream(t *testing.T) {
	proxy := NewTestProxy("test", "localhost:20009")
	proxy.Start()

	conn := AssertProxyUp(t, proxy.Listen, true)
	// Check to make sure the connection is closed
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err := conn.Read(make([]byte, 1))
	if err != io.EOF {
		t.Error("Proxy did not close connection when upstream down", err)
	}

	proxy.Stop()
}

func TestProxyBigMessage(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		buf := make([]byte, 32*1024)
		msg := make([]byte, len(buf)*2)
		hex.Encode(msg, buf)

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
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
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
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		proxy.Stop()
		proxy.Stop()
		proxy.Stop()
	})
}

func TestStartTwoProxiesOnSameAddress(t *testing.T) {
	WithTCPProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		proxy2 := NewTestProxy("proxy_2", "localhost:3306")
		proxy2.Listen = proxy.Listen
		if err := proxy2.Start(); err == nil {
			t.Fatal("Expected an err back from start")
		}
	})
}

func TestStopProxyBeforeStarting(t *testing.T) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		AssertProxyUp(t, proxy.Listen, false)

		proxy.Stop()
		err := proxy.Start()
		if err != nil {
			t.Error("Proxy failed to start", err)
		}

		err = proxy.Start()
		if err != toxiproxy.ErrProxyAlreadyStarted {
			t.Error("Proxy did not fail to start when already started", err)
		}
		AssertProxyUp(t, proxy.Listen, true)

		proxy.Stop()
		AssertProxyUp(t, proxy.Listen, false)
	})
}

func TestProxyUpdate(t *testing.T) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		err := proxy.Start()
		if err != nil {
			t.Error("Proxy failed to start", err)
		}
		AssertProxyUp(t, proxy.Listen, true)

		before := proxy.Listen

		input := &toxiproxy.Proxy{Listen: "localhost:0", Upstream: proxy.Upstream, Enabled: true}
		err = proxy.Update(input)
		if err != nil {
			t.Error("Failed to update proxy", err)
		}
		if proxy.Listen == before || proxy.Listen == input.Listen {
			t.Errorf("Proxy update didn't change listen address: %s to %s", before, proxy.Listen)
		}
		AssertProxyUp(t, proxy.Listen, true)

		input.Listen = proxy.Listen
		err = proxy.Update(input)
		if err != nil {
			t.Error("Failed to update proxy", err)
		}
		AssertProxyUp(t, proxy.Listen, true)

		input.Enabled = false
		err = proxy.Update(input)
		if err != nil {
			t.Error("Failed to update proxy", err)
		}
		AssertProxyUp(t, proxy.Listen, false)
	})
}

func TestProxyUpdateWithHostname(t *testing.T) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		err := proxy.Start()
		if err != nil {
			t.Error("Proxy failed to start", err)
		}
		AssertProxyUp(t, proxy.Listen, true)

		connectionLost := make(chan bool)

		// Start a goroutine to check if connection is maintained
		go func() {
			conn, err := net.Dial("tcp", proxy.Listen)
			if err != nil {
				t.Error("Failed to connect to proxy", err)
			}
			defer conn.Close()

			// Try to read from the connection
			buf := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, err = conn.Read(buf)
			if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				connectionLost <- true
				return
			}

			connectionLost <- false
		}()

		_, port, err := net.SplitHostPort(proxy.Listen)
		if err != nil {
			t.Error("Failed to split host and port", err)
		}

		input := &toxiproxy.Proxy{
			Listen:   net.JoinHostPort("localhost", port),
			Upstream: proxy.Upstream,
			Enabled:  true,
		}
		err = proxy.Update(input)
		if err != nil {
			t.Error("Failed to update proxy", err)
		}

		// Check if the connection was lost during the update
		if lost := <-connectionLost; lost {
			t.Error("Connection was lost during proxy update")
		}

		// Verify proxy is still up after the update
		AssertProxyUp(t, proxy.Listen, true)
	})
}

func TestRestartFailedToStartProxy(t *testing.T) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		conflict := NewTestProxy("test2", upstream)

		err := conflict.Start()
		if err != nil {
			t.Error("Proxy failed to start", err)
		}
		AssertProxyUp(t, conflict.Listen, true)

		proxy.Listen = conflict.Listen
		err = proxy.Start()
		if err == nil || err == toxiproxy.ErrProxyAlreadyStarted {
			t.Error("Proxy started when it should have conflicted")
		}

		conflict.Stop()
		AssertProxyUp(t, conflict.Listen, false)

		err = proxy.Start()
		if err != nil {
			t.Error("Proxy failed to start after conflict went away", err)
		}
		AssertProxyUp(t, proxy.Listen, true)

		proxy.Stop()
		AssertProxyUp(t, proxy.Listen, false)
	})
}

func TestProxyDiffers(t *testing.T) {
	testhelper.WithTCPServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()
		_, port, err := net.SplitHostPort(proxy.Listen)
		if err != nil {
			t.Error("Failed to split host and port", err)
		}
		otherProxy := &toxiproxy.Proxy{
			Name:     "other",
			Listen:   net.JoinHostPort("localhost", port),
			Upstream: upstream,
			Enabled:  true,
		}

		differs, err := proxy.Differs(otherProxy)
		if err != nil {
			t.Error("Failed to check if proxy differs", err)
		}
		if differs {
			t.Error("Proxy should not differ ")
		}
	})
}
