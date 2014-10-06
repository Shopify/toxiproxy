package main

import (
	"net"
	"time"
)

// Link is the TCP link between a client and an upstream.
//
// Client <-> toxiproxy <-> Upstream
//
// Its responsibility is to shove from each side to the other. Clients don't
// need to know they are talking to the upsream through toxiproxy.
type link struct {
	proxy *Proxy

	client   net.Conn
	upstream net.Conn
}

func NewLink(proxy *Proxy, client net.Conn) *link {
	return &link{
		proxy:    proxy,
		client:   client,
		upstream: &net.TCPConn{},
	}
}

func (link *link) Open() (err error) {
	link.upstream, err = net.Dial("tcp", link.proxy.Upstream)
	if err != nil {
		return err
	}

	link.pipe(link.client, link.upstream)
	link.pipe(link.upstream, link.client)

	return nil
}

func (link *link) pipe(src, dst net.Conn) {
	buf := NewStreamBuffer()
	noop := new(NoopToxic)
	noop.Init(link.proxy, src, buf)
	latency := new(LatencyToxic)
	latency.Init(link.proxy, buf, dst)
	latency.Latency = time.Millisecond * 300

	go noop.Pipe()
	go latency.Pipe()
}

func (link *link) Close() {
	link.client.Close()
	link.upstream.Close()
}
