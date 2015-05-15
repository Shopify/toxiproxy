package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"gopkg.in/tomb.v1"
)

func init() {
	logrus.SetLevel(logrus.FatalLevel)
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

func ToxicToJson(t *testing.T, name, typeName string, toxic Toxic) io.Reader {
	// Hack to add fields to an interface, json will nest otherwise, not embed
	data, err := json.Marshal(toxic)
	if err != nil {
		if t != nil {
			t.Errorf("Failed to marshal toxic for api (1): %v", toxic)
		}
		return nil
	}

	marshal := make(map[string]interface{})
	err = json.Unmarshal(data, &marshal)
	if err != nil {
		if t != nil {
			t.Errorf("Failed to unmarshal toxic (2): %v", toxic)
		}
		return nil
	}
	marshal["name"] = name
	if typeName != "" {
		marshal["type"] = typeName
	}

	data, err = json.Marshal(marshal)
	if err != nil {
		if t != nil {
			t.Errorf("Failed to marshal toxic for api (3): %v", toxic)
		}
		return nil
	}
	return bytes.NewReader(data)
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
		if upLatency == nil {
			upLatency = &LatencyToxic{}
		} else {
			_, err := proxy.upToxics.AddToxicJson(ToxicToJson(t, "", "latency", upLatency))
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		if downLatency == nil {
			downLatency = &LatencyToxic{}
		} else {
			_, err := proxy.downToxics.AddToxicJson(ToxicToJson(t, "", "latency", downLatency))
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

		proxy.upToxics.RemoveToxic("latency")
		proxy.downToxics.RemoveToxic("latency")

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}
	})
}

func TestUpstreamLatency(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Latency: 100}, nil)
}

func TestDownstreamLatency(t *testing.T) {
	DoLatencyTest(t, nil, &LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyEven(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Latency: 100}, &LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyBiasUp(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Latency: 1000}, &LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyBiasDown(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Latency: 100}, &LatencyToxic{Latency: 1000})
}

func TestZeroLatency(t *testing.T) {
	DoLatencyTest(t, &LatencyToxic{Latency: 0}, &LatencyToxic{Latency: 0})
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

	proxy.upToxics.AddToxicJson(ToxicToJson(t, "", "noop", &NoopToxic{}))
	proxy.downToxics.AddToxicJson(ToxicToJson(t, "", "noop", &NoopToxic{}))

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
		proxy.upToxics.AddToxicJson(ToxicToJson(t, "latency", "", &LatencyToxic{Latency: 10}))
		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}
		conn.Write([]byte("hello"))
		conn.Close()
		proxy.upToxics.RemoveToxic("latency")
	}
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

	proxy.downToxics.AddToxicJson(ToxicToJson(t, "latency", "", &LatencyToxic{Latency: 100}))

	time.Sleep(100 * time.Millisecond) // Wait for latency toxic
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
	proxy.upToxics.AddToxicJson(ToxicToJson(t, "", "bandwidth", &BandwidthToxic{Rate: int64(rate)}))

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

func TestSlicerToxic(t *testing.T) {
	data := []byte(strings.Repeat("hello world ", 40000)) // 480 kb
	slicer := &SlicerToxic{AverageSize: 1024, SizeVariation: 512, Delay: 10}

	input := make(chan *StreamChunk)
	output := make(chan *StreamChunk)
	stub := NewToxicStub(input, output)

	done := make(chan bool)
	go func() {
		slicer.Pipe(stub)
		done <- true
	}()
	defer func() {
		close(input)
		for {
			select {
			case <-done:
				return
			case <-output:
			}
		}
	}()

	input <- &StreamChunk{data: data}

	buf := make([]byte, 0, len(data))
	reads := 0
L:
	for {
		select {
		case c := <-output:
			reads++
			buf = append(buf, c.data...)
		case <-time.After(10 * time.Millisecond):
			break L
		}
	}

	if reads < 480/2 || reads > 480/2+480 {
		t.Errorf("Expected to read about 480 times, but read %d times.", reads)
	}
	if bytes.Compare(buf, data) != 0 {
		t.Errorf("Server did not read correct buffer from client!")
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
					proxy.upToxics.AddToxicJson(ToxicToJson(t, "", "noop", &NoopToxic{}))
					proxy.downToxics.RemoveToxic("noop")
				} else {
					proxy.upToxics.RemoveToxic("noop")
					proxy.downToxics.AddToxicJson(ToxicToJson(t, "", "noop", &NoopToxic{}))
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

	proxy.upToxics.AddToxicJson(ToxicToJson(nil, "", "bandwidth", &BandwidthToxic{Rate: 100 * 1000}))

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
