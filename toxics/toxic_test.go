package toxics_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	tomb "gopkg.in/tomb.v1"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/collectors"
	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func NewTestProxy(name, upstream string) *toxiproxy.Proxy {
	log := zerolog.Nop()
	if flag.Lookup("test.v").DefValue == "true" {
		log = zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	}
	srv := toxiproxy.NewServer(
		toxiproxy.NewMetricsContainer(prometheus.NewRegistry()),
		log,
		time.Now().UnixNano(),
	)
	srv.Metrics.ProxyMetrics = collectors.NewProxyMetricCollectors()
	proxy := toxiproxy.NewProxy(srv, name, "localhost:0", upstream)

	return proxy
}

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
				t.Error("Failed to accept client")
				return
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

func WithEchoProxy(
	t *testing.T,
	f func(proxy net.Conn, response chan []byte, proxyServer *toxiproxy.Proxy),
) {
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

func ToxicToJson(t *testing.T, name, typeName, stream string, toxic toxics.Toxic) io.Reader {
	data := map[string]interface{}{
		"name":       name,
		"type":       typeName,
		"stream":     stream,
		"attributes": toxic,
	}
	request, err := json.Marshal(data)
	if err != nil {
		t.Errorf("Failed to marshal toxic for api (1): %v", toxic)
	}

	return bytes.NewReader(request)
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
	ctx := context.Background()

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

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "noop_up", "noop", "upstream", &toxics.NoopToxic{}))
	proxy.Toxics.AddToxicJson(
		ToxicToJson(t, "noop_down", "noop", "downstream", &toxics.NoopToxic{}),
	)

	AssertEchoResponse(t, conn, serverConn)

	proxy.Toxics.ResetToxics(ctx)

	AssertEchoResponse(t, conn, serverConn)

	proxy.Toxics.ResetToxics(ctx)

	AssertEchoResponse(t, conn, serverConn)

	err = conn.Close()
	if err != nil {
		t.Error("Failed to close TCP connection", err)
	}
}

func TestToxicAddRemove(t *testing.T) {
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

	running := make(chan struct{})
	go func() {
		enabled := false
		for {
			select {
			case <-running:
				return
			default:
				if enabled {
					proxy.Toxics.AddToxicJson(
						ToxicToJson(t, "noop_up", "noop", "upstream", &toxics.NoopToxic{}),
					)
					proxy.Toxics.RemoveToxic(context.Background(), "noop_down")
				} else {
					proxy.Toxics.RemoveToxic(context.Background(), "noop_up")
					proxy.Toxics.AddToxicJson(
						ToxicToJson(t, "noop_down", "noop", "downstream", &toxics.NoopToxic{}),
					)
				}
				enabled = !enabled
			}
		}
	}()

	for i := 0; i < 100; i++ {
		AssertEchoResponse(t, conn, serverConn)
	}
	close(running)

	err = conn.Close()
	if err != nil {
		t.Error("Failed to close TCP connection", err)
	}
}

func TestProxyLatency(t *testing.T) {
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

	start := time.Now()
	for i := 0; i < 100; i++ {
		AssertEchoResponse(t, conn, serverConn)
	}
	latency := time.Since(start) / 200
	if latency > 300*time.Microsecond {
		t.Errorf("Average proxy latency > 300µs (%v)", latency)
	} else {
		t.Logf("Average proxy latency: %v", latency)
	}

	err = conn.Close()
	if err != nil {
		t.Error("Failed to close TCP connection", err)
	}
}

func BenchmarkProxyBandwidth(b *testing.B) {
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

func TestToxicStub_WriteOutput(t *testing.T) {
	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)

	buf := make([]byte, 42)
	// #nosec G404 -- used only in tests
	rand.Read(buf)

	t.Run("when no read in 1 second", func(t *testing.T) {
		err := stub.WriteOutput(&stream.StreamChunk{Data: buf}, time.Second)
		if err == nil {
			t.Error("Expected to have error")
		}

		expected := "timeout: could not write to output in 1 seconds"
		if err.Error() != expected {
			t.Errorf("Expected error: %s, got %s", expected, err)
		}
	})

	t.Run("when read is available", func(t *testing.T) {
		go func(t *testing.T, stub *toxics.ToxicStub, expected []byte) {
			select {
			case <-time.After(5 * time.Second):
				t.Error("Timeout of running test to read from output.")
			case chunk := <-output:
				if !bytes.Equal(chunk.Data, buf) {
					t.Error("Data in Output different from Write")
				}
			}
		}(t, stub, buf)

		err := stub.WriteOutput(&stream.StreamChunk{Data: buf}, 5*time.Second)
		if err != nil {
			t.Errorf("Unexpected error: %+v", err)
		}
	})
}
