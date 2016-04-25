package toxics_test

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy"
	"github.com/Shopify/toxiproxy/toxics"
)

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

func DoLatencyTest(t *testing.T, upLatency, downLatency *toxics.LatencyToxic) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		if upLatency == nil {
			upLatency = &toxics.LatencyToxic{}
		} else {
			_, err := proxy.Toxics.AddToxicJson(ToxicToJson(t, "latency_up", "latency", "upstream", upLatency))
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		if downLatency == nil {
			downLatency = &toxics.LatencyToxic{}
		} else {
			_, err := proxy.Toxics.AddToxicJson(ToxicToJson(t, "latency_down", "latency", "downstream", downLatency))
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		t.Logf("Using latency: Up: %dms +/- %dms, Down: %dms +/- %dms", upLatency.Latency, upLatency.Jitter, downLatency.Latency, downLatency.Jitter)

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
			time.Since(timer),
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
			time.Since(timer2),
			time.Duration(downLatency.Latency)*time.Millisecond,
			time.Duration(downLatency.Jitter+10)*time.Millisecond,
		)
		AssertDeltaTime(t,
			"Round trip",
			time.Since(timer),
			time.Duration(upLatency.Latency+downLatency.Latency)*time.Millisecond,
			time.Duration(upLatency.Jitter+downLatency.Jitter+10)*time.Millisecond,
		)

		proxy.Toxics.RemoveToxic("latency_up")
		proxy.Toxics.RemoveToxic("latency_down")

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestUpstreamLatency(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 100}, nil)
}

func TestDownstreamLatency(t *testing.T) {
	DoLatencyTest(t, nil, &toxics.LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyEven(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 100}, &toxics.LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyBiasUp(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 1000}, &toxics.LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyBiasDown(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 100}, &toxics.LatencyToxic{Latency: 1000})
}

func TestZeroLatency(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 0}, &toxics.LatencyToxic{Latency: 0})
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
		proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "latency", "upstream", &toxics.LatencyToxic{Latency: 10}))
		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}
		conn.Write([]byte("hello"))
		conn.Close()
		proxy.Toxics.RemoveToxic("latency")
	}
}

func TestTwoLatencyToxics(t *testing.T) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		toxics := []*toxics.LatencyToxic{&toxics.LatencyToxic{Latency: 500}, &toxics.LatencyToxic{Latency: 500}}
		for i, toxic := range toxics {
			_, err := proxy.Toxics.AddToxicJson(ToxicToJson(t, "latency_"+strconv.Itoa(i), "latency", "upstream", toxic))
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}

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
			"Upstream two latency toxics",
			time.Since(timer),
			time.Duration(1000)*time.Millisecond,
			time.Duration(10)*time.Millisecond,
		)

		for i := range toxics {
			proxy.Toxics.RemoveToxic("latency_" + strconv.Itoa(i))
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestLatencyToxicBandwidth(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}

	defer ln.Close()

	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	defer proxy.Stop()

	buf := []byte(strings.Repeat("hello world ", 1000))

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Error("Unable to accept TCP connection", err)
		}
		for err == nil {
			_, err = conn.Write(buf)
		}
	}()

	conn, err := net.Dial("tcp", proxy.Listen)
	if err != nil {
		t.Error("Unable to dial TCP server", err)
	}

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "latency", "", &toxics.LatencyToxic{Latency: 100}))

	time.Sleep(150 * time.Millisecond) // Wait for latency toxic
	buf2 := make([]byte, len(buf))

	start := time.Now()
	count := 0
	for i := 0; i < 100; i++ {
		n, err := io.ReadFull(conn, buf2)
		count += n
		if err != nil {
			t.Error(err)
			break
		}
	}

	// Assert the transfer was at least 100MB/s
	AssertDeltaTime(t, "Latency toxic bandwidth", time.Since(start), 0, time.Duration(count/100000)*time.Millisecond)

	err = conn.Close()
	if err != nil {
		t.Error("Failed to close TCP connection", err)
	}
}
