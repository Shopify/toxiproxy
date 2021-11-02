package toxics_test

import (
	"bufio"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
)

const msg = "reset toxic payload\n"

func TestResetToxicNoTimeout(t *testing.T) {
	resetTCPHelper(t, ToxicToJson(t, "resettcp", "reset_peer", "upstream", &toxics.ResetToxic{}))
}

func TestResetToxicWithTimeout(t *testing.T) {
	start := time.Now()
	resetToxic := toxics.ResetToxic{Timeout: 100}
	resetTCPHelper(t, ToxicToJson(t, "resettcp", "reset_peer", "upstream", &resetToxic))
	AssertDeltaTime(t,
		"Reset after timeout",
		time.Since(start),
		time.Duration(resetToxic.Timeout)*time.Millisecond,
		time.Duration(resetToxic.Timeout+10)*time.Millisecond,
	)
}

func TestResetToxicWithTimeoutDownstream(t *testing.T) {
	start := time.Now()
	resetToxic := toxics.ResetToxic{Timeout: 100}
	resetTCPHelper(t, ToxicToJson(t, "resettcp", "reset_peer", "downstream", &resetToxic))
	AssertDeltaTime(t,
		"Reset after timeout",
		time.Since(start),
		time.Duration(resetToxic.Timeout)*time.Millisecond,
		time.Duration(resetToxic.Timeout+10)*time.Millisecond,
	)
}

func checkConnectionState(t *testing.T, listenAddress string) {
	conn, err := net.Dial("tcp", listenAddress)
	if err != nil {
		t.Error("Unable to dial TCP server", err)
	}
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Error("Failed writing TCP payload", err)
	}
	tmp := make([]byte, 1000)
	_, err = conn.Read(tmp)
	defer conn.Close()
	if opErr, ok := err.(*net.OpError); ok {
		syscallErr, _ := opErr.Err.(*os.SyscallError)
		if !(syscallErr.Err == syscall.ECONNRESET) {
			t.Error("Expected: connection reset by peer. Got:", err)
		}
	} else {
		t.Error(
			"Expected: connection reset by peer. Got:",
			err, "conn:", conn.RemoteAddr(), conn.LocalAddr(),
		)
	}
	_, err = conn.Read(tmp)
	if err != io.EOF {
		t.Error("expected EOF from closed connection")
	}
}

func resetTCPHelper(t *testing.T, toxicJSON io.Reader) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}
	defer ln.Close()
	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	proxy.Toxics.AddToxicJson(toxicJSON)
	defer proxy.Stop()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Error("Unable to accept TCP connection", err)
		}
		defer ln.Close()
		scan := bufio.NewScanner(conn)
		if scan.Scan() {
			conn.Write([]byte(msg))
		}
	}()
	checkConnectionState(t, proxy.Listen)
}
