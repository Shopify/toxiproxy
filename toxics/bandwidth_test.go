package toxics_test

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/testhelper"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func TestBandwidthToxic(t *testing.T) {
	upstream := testhelper.NewUpstream(t, false)
	defer upstream.Close()

	proxy := NewTestProxy("test", upstream.Addr())
	proxy.Start()
	defer proxy.Stop()

	client, err := net.Dial("tcp", proxy.Listen)
	if err != nil {
		t.Fatalf("Unable to dial TCP server: %v", err)
	}

	upstreamConn := <-upstream.Connections

	rate := 1000 // 1MB/s
	proxy.Toxics.AddToxicJson(
		ToxicToJson(t, "", "bandwidth", "upstream", &toxics.BandwidthToxic{Rate: int64(rate)}),
	)

	writtenPayload := []byte(strings.Repeat("hello world ", 40000)) // 480KB
	go func() {
		n, err := client.Write(writtenPayload)
		client.Close()
		if n != len(writtenPayload) || err != nil {
			t.Errorf("Failed to write buffer: (%d == %d) %v", n, len(writtenPayload), err)
		}
	}()

	serverRecvPayload := make([]byte, len(writtenPayload))
	start := time.Now()
	_, err = io.ReadAtLeast(upstreamConn, serverRecvPayload, len(serverRecvPayload))
	if err != nil {
		t.Errorf("Proxy read failed: %v", err)
	} else if !bytes.Equal(writtenPayload, serverRecvPayload) {
		t.Errorf("Server did not read correct buffer from client!")
	}

	AssertDeltaTime(t,
		"Bandwidth",
		time.Since(start),
		time.Duration(len(writtenPayload))*time.Second/time.Duration(rate*1000),
		10*time.Millisecond,
	)
}

func BenchmarkBandwidthToxic100MB(b *testing.B) {
	upstream := testhelper.NewUpstream(b, true)
	defer upstream.Close()

	proxy := NewTestProxy("test", upstream.Addr())
	proxy.Start()
	defer proxy.Stop()

	client, err := net.Dial("tcp", proxy.Listen)
	if err != nil {
		b.Error("Unable to dial TCP server", err)
	}

	writtenPayload := []byte(strings.Repeat("hello world ", 1000))

	proxy.Toxics.AddToxicJson(
		ToxicToJson(nil, "", "bandwidth", "upstream", &toxics.BandwidthToxic{Rate: 100 * 1000}),
	)

	b.SetBytes(int64(len(writtenPayload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := client.Write(writtenPayload)
		if err != nil || n != len(writtenPayload) {
			b.Errorf("%v, %d == %d", err, n, len(writtenPayload))
			break
		}
	}

	err = client.Close()
	if err != nil {
		b.Error("Failed to close TCP connection", err)
	}
}
