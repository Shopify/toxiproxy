package toxics_test

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/stream"
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

// TestBandwidthToxicNonPositiveRate verifies that a bandwidth toxic configured
// with a non-positive rate (rate <= 0) forwards data unthrottled instead of
// hanging or panicking. The Pipe already special-cases `t.Rate <= 0` to disable
// throttling (sleep = 0), so the intent is that such a toxic is a no-op.
//
// Regression test: with rate == 0 the packet-splitting loop
// (`for len(p.Data) > t.Rate*100`) spins forever emitting empty chunks
// (p.Data[:0]) and never forwards the payload; with rate < 0 the same loop
// slices p.Data[:negative] and panics with "slice bounds out of range".
func TestBandwidthToxicNonPositiveRate(t *testing.T) {
	for _, rate := range []int64{0, -1} {
		rate := rate
		t.Run(fmt.Sprintf("rate_%d", rate), func(t *testing.T) {
			toxic := &toxics.BandwidthToxic{Rate: rate}

			input := make(chan *stream.StreamChunk)
			output := make(chan *stream.StreamChunk, 100)
			stub := toxics.NewToxicStub(input, output)

			payload := []byte("hello world")

			panicked := make(chan interface{}, 1)
			go func() {
				defer func() { panicked <- recover() }()
				toxic.Pipe(stub)
			}()

			go func() { input <- &stream.StreamChunk{Data: payload} }()

			select {
			case chunk := <-output:
				if !bytes.Equal(chunk.Data, payload) {
					t.Errorf("rate=%d: expected full payload forwarded unthrottled, "+
						"got %d bytes (%q)", rate, len(chunk.Data), chunk.Data)
				}
			case p := <-panicked:
				t.Fatalf("rate=%d: BandwidthToxic.Pipe panicked: %v", rate, p)
			case <-time.After(2 * time.Second):
				t.Fatalf("rate=%d: timed out; BandwidthToxic.Pipe did not forward "+
					"the chunk (infinite split loop)", rate)
			}

			// Unblock and stop the Pipe goroutine if it is still running.
			select {
			case stub.Interrupt <- struct{}{}:
			case <-time.After(time.Second):
			}
		})
	}
}
