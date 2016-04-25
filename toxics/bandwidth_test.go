package toxics_test

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/toxics"
)

func TestBandwidthToxic(t *testing.T) {
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

	rate := 1000 // 1MB/s
	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "bandwidth", "upstream", &toxics.BandwidthToxic{Rate: int64(rate)}))

	buf := []byte(strings.Repeat("hello world ", 40000)) // 480KB
	go func() {
		n, err := conn.Write(buf)
		conn.Close()
		if n != len(buf) || err != nil {
			t.Errorf("Failed to write buffer: (%d == %d) %v", n, len(buf), err)
		}
	}()

	buf2 := make([]byte, len(buf))
	start := time.Now()
	_, err = io.ReadAtLeast(serverConn, buf2, len(buf2))
	if err != nil {
		t.Errorf("Proxy read failed: %v", err)
	} else if bytes.Compare(buf, buf2) != 0 {
		t.Errorf("Server did not read correct buffer from client!")
	}

	AssertDeltaTime(t,
		"Bandwidth",
		time.Since(start),
		time.Duration(len(buf))*time.Second/time.Duration(rate*1000),
		10*time.Millisecond,
	)
}

func BenchmarkBandwidthToxic100MB(b *testing.B) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatal("Failed to create TCP server", err)
	}

	defer ln.Close()

	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	defer proxy.Stop()

	buf := []byte(strings.Repeat("hello world ", 1000))

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			b.Error("Unable to accept TCP connection", err)
		}
		buf2 := make([]byte, len(buf))
		for err == nil {
			_, err = conn.Read(buf2)
		}
	}()

	conn, err := net.Dial("tcp", proxy.Listen)
	if err != nil {
		b.Error("Unable to dial TCP server", err)
	}

	proxy.Toxics.AddToxicJson(ToxicToJson(nil, "", "bandwidth", "upstream", &toxics.BandwidthToxic{Rate: 100 * 1000}))

	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := conn.Write(buf)
		if err != nil || n != len(buf) {
			b.Errorf("%v, %d == %d", err, n, len(buf))
			break
		}
	}

	err = conn.Close()
	if err != nil {
		b.Error("Failed to close TCP connection", err)
	}
}
