package toxics_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/testhelper"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func AssertDeltaTime(t *testing.T, message string, actual, expected, delta time.Duration) {
	diff := actual - expected
	if diff < 0 {
		diff *= -1
	}
	if diff > delta {
		t.Errorf(
			"[%s] Time was more than %v off: got %v expected %v",
			message,
			delta,
			actual,
			expected,
		)
	} else {
		t.Logf("[%s] Time was correct: %v (expected %v)", message, actual, expected)
	}
}

func DoLatencyTest(t *testing.T, upLatency, downLatency *toxics.LatencyToxic) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		if upLatency == nil {
			upLatency = &toxics.LatencyToxic{}
		} else {
			_, err := proxy.Toxics.AddToxicJson(
				ToxicToJson(t, "latency_up", "latency", "upstream", upLatency),
			)
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		if downLatency == nil {
			downLatency = &toxics.LatencyToxic{}
		} else {
			_, err := proxy.Toxics.AddToxicJson(
				ToxicToJson(t, "latency_down", "latency", "downstream", downLatency),
			)
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		t.Logf(
			"Using latency: Up: %dms +/- %dms, Down: %dms +/- %dms",
			upLatency.Latency,
			upLatency.Jitter,
			downLatency.Latency,
			downLatency.Jitter,
		)

		// Expecting the same latency in both rounds
		doLatencyRound(t, conn, response, upLatency.Latency, downLatency.Latency, upLatency.Jitter, downLatency.Jitter)
		doLatencyRound(t, conn, response, upLatency.Latency, downLatency.Latency, upLatency.Jitter, downLatency.Jitter)

		ctx := context.Background()
		proxy.Toxics.RemoveToxic(ctx, "latency_up")
		proxy.Toxics.RemoveToxic(ctx, "latency_down")

		err := conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func doLatencyRound(t *testing.T, conn net.Conn, response chan []byte, upLatency, downLatency, upJitter, downJitter int64) {
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
		time.Duration(upLatency)*time.Millisecond,
		time.Duration(upJitter+10)*time.Millisecond,
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
		time.Duration(downLatency)*time.Millisecond,
		time.Duration(downJitter+10)*time.Millisecond,
	)
	AssertDeltaTime(t,
		"Round trip",
		time.Since(timer),
		time.Duration(upLatency+downLatency)*time.Millisecond,
		time.Duration(upJitter+downJitter+20)*time.Millisecond,
	)
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
		proxy.Toxics.AddToxicJson(
			ToxicToJson(t, "", "latency", "upstream", &toxics.LatencyToxic{Latency: 10}),
		)
		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}
		conn.Write([]byte("hello"))
		conn.Close()
		proxy.Toxics.RemoveToxic(context.Background(), "latency")
	}
}

func TestTwoLatencyToxics(t *testing.T) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		toxics := []*toxics.LatencyToxic{{Latency: 500}, {Latency: 500}}
		for i, toxic := range toxics {
			_, err := proxy.Toxics.AddToxicJson(
				ToxicToJson(t, "latency_"+strconv.Itoa(i), "latency", "upstream", toxic),
			)
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
			proxy.Toxics.RemoveToxic(context.Background(), "latency_"+strconv.Itoa(i))
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestLatencyToxicBandwidth(t *testing.T) {
	upstream := testhelper.NewUpstream(t, false)
	defer upstream.Close()

	proxy := NewTestProxy("test", upstream.Addr())
	proxy.Start()
	defer proxy.Stop()

	client, err := net.Dial("tcp", proxy.Listen)
	if err != nil {
		t.Fatalf("Unable to dial TCP server: %v", err)
	}

	writtenPayload := []byte(strings.Repeat("hello world ", 1000))
	upstreamConn := <-upstream.Connections
	go func(conn net.Conn, payload []byte) {
		var err error
		for err == nil {
			_, err = conn.Write(payload)
		}
	}(upstreamConn, writtenPayload)

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "latency", "", &toxics.LatencyToxic{Latency: 100}))

	time.Sleep(150 * time.Millisecond) // Wait for latency toxic
	response := make([]byte, len(writtenPayload))

	start := time.Now()
	count := 0
	for i := 0; i < 100; i++ {
		n, err := io.ReadFull(client, response)
		if err != nil {
			t.Fatalf("Could not read from socket: %v", err)
			break
		}
		count += n
	}

	// Assert the transfer was at least 100MB/s
	AssertDeltaTime(
		t,
		"Latency toxic bandwidth",
		time.Since(start),
		0,
		time.Duration(count/100000)*time.Millisecond,
	)

	err = client.Close()
	if err != nil {
		t.Error("Failed to close TCP connection", err)
	}
}
