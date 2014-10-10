package main

import (
	"io"
	"net"
)

// Link is the TCP link between a client and an upstream.
//
// Client <-> toxiproxy <-> Upstream
//
// Its responsibility is to shove from each side to the other. Clients don't
// need to know they are talking to the upsream through toxiproxy.
type ProxyLink struct {
	proxy *Proxy

	client   net.Conn
	upstream net.Conn

	upToxics    []*ToxicStub
	downToxics  []*ToxicStub
	upBuffers   []*Pipe
	downBuffers []*Pipe
}

type Pipe struct {
	io.Reader
	io.WriteCloser
}

func NewPipe() *Pipe {
	r, w := io.Pipe()
	return &Pipe{r, w}
}

func NewLink(proxy *Proxy, client net.Conn, upstream net.Conn) *ProxyLink {
	link := &ProxyLink{
		proxy:       proxy,
		client:      client,
		upstream:    upstream,
		upToxics:    make([]*ToxicStub, MaxToxics),
		downToxics:  make([]*ToxicStub, MaxToxics),
		upBuffers:   make([]*Pipe, MaxToxics-1),
		downBuffers: make([]*Pipe, MaxToxics-1),
	}

	for i := 0; i < MaxToxics-1; i++ {
		link.upBuffers[i] = NewPipe()
		link.downBuffers[i] = NewPipe()
		if i > 0 {
			// Initialize all toxics that only connect through the buffers
			link.upToxics[i] = NewToxicStub(proxy, link.upBuffers[i-1], link.upBuffers[i])
			link.downToxics[i] = NewToxicStub(proxy, link.downBuffers[i-1], link.downBuffers[i])
		}
	}
	// Initialize the first and last toxics with the client and upstream
	if MaxToxics > 1 {
		link.upToxics[0] = NewToxicStub(proxy, client, link.upBuffers[0])
		link.downToxics[0] = NewToxicStub(proxy, upstream, link.downBuffers[0])
		last := MaxToxics - 1 // To stop compile errors from MaxToxics-2 if MaxToxics == 1
		link.upToxics[last] = NewToxicStub(proxy, link.upBuffers[last-1], upstream)
		link.downToxics[last] = NewToxicStub(proxy, link.downBuffers[last-1], client)
	} else {
		link.upToxics[0] = NewToxicStub(proxy, client, upstream)
		link.downToxics[0] = NewToxicStub(proxy, upstream, client)
	}

	// Start all the toxics with the current configuration
	for i := 0; i < MaxToxics; i++ {
		go proxy.toxics.uplink[i].Pipe(link.upToxics[i])
		go proxy.toxics.downlink[i].Pipe(link.downToxics[i])
	}
	return link
}

func (link *ProxyLink) SetUpstreamToxic(toxic Toxic, index int) {
	link.upToxics[index].Interrupt()
	go toxic.Pipe(link.upToxics[index])
}

func (link *ProxyLink) SetDownstreamToxic(toxic Toxic, index int) {
	link.downToxics[index].Interrupt()
	go toxic.Pipe(link.downToxics[index])
}

func (link *ProxyLink) Close() {
	link.client.Close()
	link.upstream.Close()
}
