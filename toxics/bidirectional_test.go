package toxics_test

import (
	"bufio"
	"bytes"
	"math/rand"
	"net"
	"strconv"
	"testing"

	"github.com/Shopify/toxiproxy"
	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
)

type EchoToxic struct {
	Replace bool `json:"replace"`
}

type EchoToxicState struct {
	Request chan *stream.StreamChunk
}

func (t *EchoToxic) PipeRequest(stub *toxics.ToxicStub) {
	state := stub.State.(*EchoToxicState)

	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				close(state.Request)
				stub.Close()
				return
			}
			state.Request <- c
		}
	}
}

func (t *EchoToxic) Pipe(stub *toxics.ToxicStub) {
	state := stub.State.(*EchoToxicState)

	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-state.Request:
			if c == nil {
				stub.Close()
				return
			}
			if t.Replace {
				c.Data = []byte("foobar\n")
			}
			stub.Output <- c
		}
	}
}

func (t *EchoToxic) NewState() interface{} {
	return &EchoToxicState{
		Request: make(chan *stream.StreamChunk),
	}
}

func init() {
	toxics.Register("echo_test", new(EchoToxic))
}

func WithEchoToxic(t *testing.T, existingLink bool, f func(proxy net.Conn, response chan []byte, proxyServer *toxiproxy.Proxy)) {
	WithEchoServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()

		// Test the 2 different code paths for adding toxics to existing links vs adding them on link creation.
		if !existingLink {
			proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "echo_test", "both", &EchoToxic{}))
		}

		conn, err := net.Dial("tcp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}

		if existingLink {
			proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "echo_test", "both", &EchoToxic{}))
		}

		f(conn, response, proxy)

		proxy.Stop()
	})
}

func AssertToxicEchoResponse(t *testing.T, proxy net.Conn, serverResponse chan []byte, expectServer bool) {
	msg := []byte("hello " + strconv.Itoa(rand.Int()) + " world\n")

	_, err := proxy.Write(msg)
	if err != nil {
		t.Error("Failed writing to TCP server", err)
	}

	scan := bufio.NewScanner(proxy)
	if !scan.Scan() {
		t.Error("Server unexpectedly closed connection")
	}

	resp := append(scan.Bytes(), '\n')
	if !bytes.Equal(resp, msg) {
		t.Error("Client didn't read correct bytes from proxy:", string(resp), "!=", string(msg))
	}

	if expectServer {
		resp = <-serverResponse
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read correct bytes from client:", string(resp), "!=", string(msg))
		}
	} else {
		select {
		case resp = <-serverResponse:
			t.Error("Server got unexpected data from client:", string(resp))
		default:
		}
	}
}

func TestAddUpdateRemoveBidirectionalToxic(t *testing.T) {
	for existing := 0; existing < 2; existing++ {
		WithEchoToxic(t, existing > 0, func(proxy net.Conn, response chan []byte, proxyServer *toxiproxy.Proxy) {
			AssertToxicEchoResponse(t, proxy, response, false)

			proxyServer.Toxics.UpdateToxicJson("echo_test", bytes.NewReader([]byte(`{"attributes": {"replace": true}}`)))

			_, err := proxy.Write([]byte("hello world\n"))
			if err != nil {
				t.Error("Failed writing to TCP server", err)
			}

			scan := bufio.NewScanner(proxy)
			if !scan.Scan() {
				t.Error("Server unexpectedly closed connection")
			}

			resp := scan.Bytes()
			if !bytes.Equal(resp, []byte("foobar")) {
				t.Error("Client didn't read correct bytes from proxy:", string(resp), "!= foobar")
			}

			proxyServer.Toxics.RemoveToxic("echo_test")

			AssertToxicEchoResponse(t, proxy, response, true)
		})
	}
}

func TestBidirectionalToxicOnlyShowsUpOnce(t *testing.T) {
	proxy := NewTestProxy("test", "localhost:20001")
	proxy.Start()

	toxic, _ := proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "echo_test", "both", &EchoToxic{}))
	if toxic.PairedToxic == nil {
		t.Fatal("Expected bidirectional toxic to have a paired toxic.")
	} else if toxic.PairedToxic.Name != "" || toxic.PairedToxic.Direction != stream.Upstream || toxic.PairedToxic.PairedToxic != toxic {
		t.Fatalf("Paired toxic had incorrect values: %+v", toxic.PairedToxic)
	} else if toxic.Direction != stream.Downstream {
		t.Fatal("Expected toxic to have downstream direction set:", toxic.Direction)
	}

	toxics := proxy.Toxics.GetToxicArray()
	if len(toxics) != 1 {
		t.Fatalf("Wrong number of toxics returned: %d != 1", len(toxics))
	} else if toxics[0].Name != "echo_test" || toxics[0].Stream != "both" {
		t.Fatalf("Toxic was not stored as expected: %+v", toxics[0])
	}

	proxy.Stop()
}

func TestBidirectionalToxicDuplicateName(t *testing.T) {
	proxy := NewTestProxy("test", "localhost:20001")
	proxy.Start()

	// Test against regular toxics
	proxy.Toxics.AddToxicJson(ToxicToJson(t, "foo", "latency", "downstream", &toxics.LatencyToxic{}))
	_, err := proxy.Toxics.AddToxicJson(ToxicToJson(t, "foo", "echo_test", "both", &EchoToxic{}))
	if err != toxiproxy.ErrToxicAlreadyExists {
		t.Fatal("Expected adding toxic to fail due to existing toxic:", err)
	}

	// Test against other bidirection toxics
	proxy.Toxics.AddToxicJson(ToxicToJson(t, "bar", "echo_test", "both", &EchoToxic{}))
	_, err = proxy.Toxics.AddToxicJson(ToxicToJson(t, "bar", "echo_test", "both", &EchoToxic{}))
	if err != toxiproxy.ErrToxicAlreadyExists {
		t.Fatal("Expected adding toxic to fail due to existing bidirectional toxic:", err)
	}

	toxics := proxy.Toxics.GetToxicArray()
	if len(toxics) != 2 {
		t.Fatalf("Wrong number of toxics returned: %d != 2", len(toxics))
	} else if toxics[0].Name != "foo" || toxics[0].Type != "latency" || toxics[0].Stream != "downstream" {
		t.Fatalf("Latency toxic was not stored as expected: %+v", toxics[0])
	} else if toxics[1].Name != "bar" || toxics[1].Type != "echo_test" || toxics[1].Stream != "both" {
		t.Fatalf("Bidirectional toxic was not stored as expected: %+v", toxics[1])
	}

	proxy.Stop()
}

// TODO: Test toxicity on bidirectional toxics
