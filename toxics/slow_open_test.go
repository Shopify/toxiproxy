package toxics_test

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/testhelper"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func DoSlowOpenTest(t *testing.T, upSlowOpen, downSlowOpen *toxics.SlowOpenToxic) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		if upSlowOpen == nil {
			upSlowOpen = &toxics.SlowOpenToxic{}
		} else {
			_, err := proxy.Toxics.AddToxicJson(
				ToxicToJson(t, "slow_open_up", "slow_open", "upstream", upSlowOpen),
			)
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		if downSlowOpen == nil {
			downSlowOpen = &toxics.SlowOpenToxic{}
		} else {
			_, err := proxy.Toxics.AddToxicJson(
				ToxicToJson(t, "slow_open_down", "slow_open", "downstream", downSlowOpen),
			)
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		t.Logf(
			"Using slow_open: Up: %dms, Down: %dms",
			upSlowOpen.Delay,
			downSlowOpen.Delay,
		)

		// First round: expecting delay
		doLatencyRound(t, conn, response, upSlowOpen.Delay, downSlowOpen.Delay, 0, 0)
		// Second and third rounds: not expecting delay
		doLatencyRound(t, conn, response, 0, 0, 0, 0)

		ctx := context.Background()
		proxy.Toxics.RemoveToxic(ctx, "slow_open_up")
		proxy.Toxics.RemoveToxic(ctx, "slow_open_down")

		err := conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestUpstreamSlowOpen(t *testing.T) {
	DoSlowOpenTest(t, &toxics.SlowOpenToxic{Delay: 100}, nil)
}

func TestDownstreamSlowOpen(t *testing.T) {
	DoSlowOpenTest(t, nil, &toxics.SlowOpenToxic{Delay: 100})
}

func TestFullstreamSlowOpenEven(t *testing.T) {
	DoSlowOpenTest(t, &toxics.SlowOpenToxic{Delay: 100}, &toxics.SlowOpenToxic{Delay: 100})
}

func TestFullstreamSlowOpenBiasUp(t *testing.T) {
	DoSlowOpenTest(t, &toxics.SlowOpenToxic{Delay: 1000}, &toxics.SlowOpenToxic{Delay: 100})
}

func TestFullstreamSlowOpenBiasDown(t *testing.T) {
	DoSlowOpenTest(t, &toxics.SlowOpenToxic{Delay: 100}, &toxics.SlowOpenToxic{Delay: 1000})
}

func TestZeroDelay(t *testing.T) {
	DoSlowOpenTest(t, &toxics.SlowOpenToxic{Delay: 0}, &toxics.SlowOpenToxic{Delay: 0})
}

func TestSlowOpenToxicCloseRace(t *testing.T) {
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
			ToxicToJson(t, "", "slow_open", "upstream", &toxics.SlowOpenToxic{Delay: 10}),
		)
		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}
		conn.Write([]byte("hello"))
		conn.Close()
		proxy.Toxics.RemoveToxic(context.Background(), "slow_open")
	}
}

func TestSlowOpenToxicWithLatencyToxic(t *testing.T) {
	const delay = 500

	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		var err error
		_, err = proxy.Toxics.AddToxicJson(
			ToxicToJson(t, "slow_open", "slow_open", "upstream", &toxics.SlowOpenToxic{
				Delay: delay,
			}),
		)
		if err != nil {
			t.Error("AddToxicJson returned error:", err)
		}
		_, err = proxy.Toxics.AddToxicJson(
			ToxicToJson(t, "latency", "latency", "upstream", &toxics.LatencyToxic{
				Latency: delay,
			}),
		)
		if err != nil {
			t.Error("AddToxicJson returned error:", err)
		}

		// First round: expecting double delay (SlowOpen + Latency)
		doLatencyRound(t, conn, response, delay+delay, 0, 0, 0)

		// Second and third rounds: expecting single delay (Latency only)
		doLatencyRound(t, conn, response, 0+delay, 0, 0, 0)
		doLatencyRound(t, conn, response, 0+delay, 0, 0, 0)

		ctx := context.Background()
		proxy.Toxics.RemoveToxic(ctx, "slow_open")
		proxy.Toxics.RemoveToxic(ctx, "latency")

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestSlowOpenToxicBandwidth(t *testing.T) {
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

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "slow_open", "", &toxics.SlowOpenToxic{Delay: 100}))

	time.Sleep(150 * time.Millisecond) // Wait for slow_open toxic
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
