package testhelper

import (
	"io/ioutil"
	"net"
	"testing"
)

func NewTCPServer() (*TCPServer, error) {
	result := &TCPServer{
		addr:     "localhost:0",
		response: make(chan []byte, 1),
	}
	err := result.Run()
	if err != nil {
		return nil, err
	}
	return result, nil
}

type TCPServer struct {
	addr     string
	server   net.Listener
	response chan []byte
}

func (server *TCPServer) Run() (err error) {
	server.server, err = net.Listen("tcp", server.addr)
	if err != nil {
		return
	}
	server.addr = server.server.Addr().String()
	return
}

func (server *TCPServer) handle_connection() (err error) {
	conn, err := server.server.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	val, err := ioutil.ReadAll(conn)
	if err != nil {
		return
	}

	server.response <- val
	return
}

func (server *TCPServer) Close() (err error) {
	return server.server.Close()
}

func WithTCPServer(t *testing.T, block func(string, chan []byte)) {
	server, err := NewTCPServer()
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}
	go func(t *testing.T, server *TCPServer) {
		err := server.handle_connection()
		if err != nil {
			t.Error("Failed to handle connection", err)
		}
	}(t, server)
	block(server.addr, server.response)
}
