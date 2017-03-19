package toxics_test

import (
  "testing"
  "net"
  "time"

	"github.com/Shopify/toxiproxy/toxics"
)

func TestTimeoutToxicDoesNotCauseHang(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}
	defer ln.Close()

	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	defer proxy.Stop()

  proxy.Toxics.AddToxicJson(ToxicToJson(t, "might_block", "latency", "upstream", &toxics.LatencyToxic{Latency: 10}))
  proxy.Toxics.AddToxicJson(ToxicToJson(t, "to_delete", "timeout", "upstream", &toxics.TimeoutToxic{Timeout: 0}))

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
    t.Fatal("Unable to dial TCP server", err)
  }
  defer conn.Close()

	_ = <-serverConnRecv

  _, err = conn.Write([]byte("hello"))
  if err != nil {
    t.Fatal("Unable to write to proxy", err)
  }

  time.Sleep(1 * time.Second) // Shitty sync waiting for latency toxi to get data.

  done := make(chan struct{})
  go func() {
    proxy.Toxics.RemoveToxic("might_block")
    close(done)
  }()

  select {
  case <-done:
  case <-time.After(1 * time.Second):
    t.Fatal("timeout toxic is causing latency toxic to block")
  }
}

