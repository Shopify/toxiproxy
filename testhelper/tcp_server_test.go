package testhelper_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/Shopify/toxiproxy/v2/testhelper"
)

func TestSimpleServer(t *testing.T) {
	testhelper.WithTCPServer(t, func(addr string, response chan []byte) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Error("Unable to dial TCP server", err)
		}

		msg := []byte("hello world")

		_, err = conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to TCP server", err)
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close TCP connection", err)
		}

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read bytes from client")
		}
	})
}
