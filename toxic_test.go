package main

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"gopkg.in/tomb.v1"
)

func WithEchoServer(t *testing.T, f func(string, chan []byte)) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}

	defer ln.Close()

	response := make(chan []byte, 1)
	tomb := tomb.Tomb{}

	go func() {
		defer tomb.Done()
		src, err := ln.Accept()
		if err != nil {
			select {
			case <-tomb.Dying():
			default:
				t.Fatal("Failed to accept client")
			}
			return
		}

		ln.Close()

		scan := bufio.NewScanner(src)
		if scan.Scan() {
			received := append(scan.Bytes(), '\n')
			response <- received

			src.Write(received)
		}
	}()

	f(ln.Addr().String(), response)

	tomb.Killf("Function body finished")
	ln.Close()
	tomb.Wait()

	close(response)
}

func WithEchoProxy(t *testing.T, f func(proxy net.Conn, response chan []byte, proxyServer *Proxy)) {
	WithEchoServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()

		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}

		f(conn, response, proxy)

		proxy.Stop()
	})
}

func AssertDeltaTime(t *testing.T, message string, actual, expected, delta time.Duration) {
	diff := actual - expected
	if diff < 0 {
		diff *= -1
	}
	if diff > delta {
		t.Errorf("[%s] Time was more than %v off: got %v expected %v", message, delta, actual, expected)
	} else {
		t.Logf("[%s] Time was correct: %v (expected %v)", message, actual, expected)
	}
}

func DoLatencyTest(t *testing.T, upLatency, downLatency *LatencyToxic) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *Proxy) {
		t.Logf("Using latency: Up: %dms +/- %dms, Down: %dms +/- %dms", upLatency.Latency, upLatency.Jitter, downLatency.Latency, downLatency.Jitter)
		proxy.upToxics.SetToxicValue(upLatency)
		proxy.downToxics.SetToxicValue(downLatency)

		msg := []byte("hello world " + strings.Repeat("a", 32*1024) + "\n")

		timer := time.Now()
		_, err := conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to TCP server", err)
		}

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read correct bytes from client:", string(resp))
		}
		AssertDeltaTime(t,
			"Server read",
			time.Now().Sub(timer),
			time.Duration(upLatency.Latency)*time.Millisecond,
			time.Duration(upLatency.Jitter+10)*time.Millisecond,
		)
		timer2 := time.Now()

		scan := bufio.NewScanner(conn)
		if scan.Scan() {
			resp = append(scan.Bytes(), '\n')
			if !bytes.Equal(resp, msg) {
				t.Error("Client didn't read correct bytes from server:", string(resp))
			}
		}
		AssertDeltaTime(t,
			"Client read",
			time.Now().Sub(timer2),
			time.Duration(downLatency.Latency)*time.Millisecond,
			time.Duration(downLatency.Jitter+10)*time.Millisecond,
		)
		AssertDeltaTime(t,
			"Round trip",
			time.Now().Sub(timer),
			time.Duration(upLatency.Latency+downLatency.Latency)*time.Millisecond,
			time.Duration(upLatency.Jitter+downLatency.Jitter+10)*time.Millisecond,
		)

		upLatency.Enabled = false
		downLatency.Enabled = false
		proxy.upToxics.SetToxicValue(upLatency)
		proxy.downToxics.SetToxicValue(downLatency)

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestUpstreamLatency(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Enabled: true, Latency: 100}, &LatencyToxic{Enabled: false})
}

func TestDownstreamLatency(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Enabled: false}, &LatencyToxic{Enabled: true, Latency: 100})
}

func TestFullstreamLatencyEven(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Enabled: true, Latency: 100}, &LatencyToxic{Enabled: true, Latency: 100})
}

func TestFullstreamLatencyBiasUp(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Enabled: true, Latency: 1000}, &LatencyToxic{Enabled: true, Latency: 100})
}

func TestFullstreamLatencyBiasDown(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Enabled: true, Latency: 100}, &LatencyToxic{Enabled: true, Latency: 1000})
}

func TestZeroLatency(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Enabled: true, Latency: 0}, &LatencyToxic{Enabled: true, Latency: 0})
}

func AssertEchoResponse(t *testing.T, client, server net.Conn) {
	msg := []byte("hello world\n")

	_, err := client.Write(msg)
	if err != nil {
		t.Error("Failed writing to TCP server", err)
	}

	scan := bufio.NewScanner(server)
	if !scan.Scan() {
		t.Error("Client unexpectedly closed connection")
	}

	resp := append(scan.Bytes(), '\n')
	if !bytes.Equal(resp, msg) {
		t.Error("Server didn't read correct bytes from client:", string(resp))
	}

	_, err = server.Write(resp)
	if err != nil {
		t.Error("Failed writing to TCP client", err)
	}

	scan = bufio.NewScanner(client)
	if !scan.Scan() {
		t.Error("Server unexpectedly closed connection")
	}

	resp = append(scan.Bytes(), '\n')
	if !bytes.Equal(resp, msg) {
		t.Error("Client didn't read correct bytes from server:", string(resp))
	}
}

func TestPersistentConnections(t *testing.T) {
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
		t.Error("Unable to dial TCP server", err)
	}

	serverConn := <-serverConnRecv

	proxy.upToxics.SetToxicValue(&LatencyToxic{Enabled: true, Latency: 0})
	proxy.downToxics.SetToxicValue(&LatencyToxic{Enabled: true, Latency: 0})

	AssertEchoResponse(t, conn, serverConn)

	proxy.upToxics.ResetToxics()
	proxy.downToxics.ResetToxics()

	AssertEchoResponse(t, conn, serverConn)

	proxy.upToxics.ResetToxics()
	proxy.downToxics.ResetToxics()

	AssertEchoResponse(t, conn, serverConn)

	err = conn.Close()
	if err != nil {
		t.Error("Failed to close TCP connection", err)
	}
}

func TestLatencyToxicCloseRace(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}

	defer ln.Close()

	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	defer proxy.Stop()

	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	// Check for potential race conditions when interrupting toxics
	for i := 0; i < 1000; i++ {
		proxy.upToxics.SetToxicValue(&LatencyToxic{Enabled: true, Latency: 10})
		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}
		conn.Write([]byte("hello"))
		conn.Close()
		proxy.upToxics.SetToxicValue(&LatencyToxic{Enabled: false})
	}
}
