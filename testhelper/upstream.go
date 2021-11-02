package testhelper

import (
	"net"
	"testing"
)

type Upstream struct {
	listener    net.Listener
	logger      testing.TB
	Connections chan net.Conn
}

func NewUpstream(t testing.TB, ignoreData bool) *Upstream {
	result := &Upstream{
		logger: t,
	}

	result.listen()
	result.accept(ignoreData)

	return result
}

func (u *Upstream) listen() {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		u.logger.Fatalf("Failed to create TCP server: %v", err)
	}
	u.listener = listener
}

func (u *Upstream) accept(ignoreData bool) {
	u.Connections = make(chan net.Conn)
	go func(u *Upstream) {
		conn, err := u.listener.Accept()
		if err != nil {
			u.logger.Fatalf("Unable to accept TCP connection: %v", err)
		}
		if ignoreData {
			buf := make([]byte, 4000)
			for err == nil {
				_, err = conn.Read(buf)
			}
		} else {
			u.Connections <- conn
		}
	}(u)
}

func (u *Upstream) Close() {
	u.listener.Close()
}

func (u *Upstream) Addr() string {
	return u.listener.Addr().String()
}
