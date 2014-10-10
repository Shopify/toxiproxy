package main

import (
	"io"
	"net"
)

// ProxyLink is a TCP link between a client and an upstream.
// It is for a single direction only, so 2 ProxyLinks exist per TCP connection.
//
// Client <-> toxiproxy <-> Upstream
//
// Its responsibility is to shove from one side to the other. Clients don't
// need to know they are talking to the upsream through toxiproxy.
type ProxyLink struct {
	proxy *Proxy

	input  net.Conn
	output net.Conn

	stream []*ToxicStub
}

func NewLink(proxy *Proxy, input net.Conn, output net.Conn) *ProxyLink {
	link := &ProxyLink{
		proxy:  proxy,
		input:  input,
		output: output,
		stream: make([]*ToxicStub, MaxToxics),
	}

	// Initialize the stream with ToxicStubs
	var last io.Reader = input
	for i := 0; i < MaxToxics; i++ {
		if i == MaxToxics-1 {
			link.stream[i] = NewToxicStub(proxy, last, output)
		} else {
			r, w := io.Pipe()
			link.stream[i] = NewToxicStub(proxy, last, w)
			last = r
		}
	}
	return link
}

// Start the stream with the specified toxics
func (link *ProxyLink) Start(toxics []Toxic) {
	for i, toxic := range toxics {
		go toxic.Pipe(link.stream[i])
	}
}

// Replace the toxic at the specified index
func (link *ProxyLink) SetToxic(toxic Toxic, index int) {
	link.stream[index].Interrupt()
	go toxic.Pipe(link.stream[index])
}

func (link *ProxyLink) Close() {
	link.input.Close()
	link.output.Close()
}
