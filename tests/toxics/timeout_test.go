package toxics

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/pkg/proxy"
	"github.com/Shopify/toxiproxy/pkg/toxics"
)

func WithEstablishedProxy(t *testing.T, f func(net.Conn, net.Conn, *proxy.Proxy)) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}
	defer ln.Close()

	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	defer proxy.Stop()

	serverConnRecv := make(chan net.Conn)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Error("Unable to accept TCP connection", err)
		}
		serverConnRecv <- conn
	}()

	conn, err := net.Dial("tcp", proxy.Listen)
	if err != nil {
		t.Fatal("Unable to dial TCP server", err)
	}
	defer conn.Close()

	serverConn := <-serverConnRecv
	defer serverConn.Close()

	writeAndReceive := func(from, to net.Conn) {
		data := []byte("foobar")
		_, err := from.Write(data)
		if err != nil {
			t.Fatal(err)
		}

		err = TimeoutAfter(time.Second, func() {
			resp := make([]byte, len(data))
			to.Read(resp)
			if !bytes.Equal(resp, data) {
				t.Fatalf("expected '%s' but got '%s'", string(data), string(resp))
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Make sure we can send and receive data before continuing.
	writeAndReceive(conn, serverConn)
	writeAndReceive(serverConn, conn)

	f(conn, serverConn, proxy)
}

func TestTimeoutToxicDoesNotCauseHang(t *testing.T) {
	WithEstablishedProxy(t, func(conn, _ net.Conn, proxy *proxy.Proxy) {
		proxy.Toxics.AddToxicJson(ToxicToJson(t, "might_block", "latency", "upstream", &toxics.Latency{Latency: 10}))
		proxy.Toxics.AddToxicJson(ToxicToJson(t, "timeout", "timeout", "upstream", &toxics.Timeout{Timeout: 0}))

		for i := 0; i < 5; i++ {
			_, err := conn.Write([]byte("hello"))
			if err != nil {
				t.Fatal("Unable to write to proxy", err)
			}
			time.Sleep(200 * time.Millisecond) // Shitty sync waiting for latency toxi to get data.
		}

		err := TimeoutAfter(time.Second, func() {
			proxy.Toxics.RemoveToxic("might_block")
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestTimeoutToxicClosesConnectionOnRemove(t *testing.T) {
	WithEstablishedProxy(t, func(conn, serverConn net.Conn, proxy *proxy.Proxy) {
		proxy.Toxics.AddToxicJson(ToxicToJson(t, "to_delete", "timeout", "upstream", &toxics.Timeout{Timeout: 0}))

		proxy.Toxics.RemoveToxic("to_delete")

		err := TimeoutAfter(time.Second, func() {
			buf := make([]byte, 1)
			_, err := conn.Read(buf)
			if err != io.EOF {
				t.Fatal("expected EOF from closed connetion")
			}
			_, err = serverConn.Read(buf)
			if err != io.EOF {
				t.Fatal("expected EOF from closed server connetion")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TimeoutAfter(after time.Duration, f func()) error {
	success := make(chan struct{})
	go func() {
		f()
		close(success)
	}()
	select {
	case <-success:
		return nil
	case <-time.After(after):
		return fmt.Errorf("timed out after %s", after)
	}
}
