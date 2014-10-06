package main

import "net"

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
	buf1 := NewStreamBuffer()
	buf2 := NewStreamBuffer()
	buf3 := NewStreamBuffer()
	buf4 := NewStreamBuffer()
	noop1 := new(NoopToxic)
	noop2 := new(NoopToxic)
	noop3 := new(NoopToxic)
	noop4 := new(NoopToxic)
	noop5 := new(NoopToxic)
	noop1.Init(link.proxy, src, buf1)
	noop2.Init(link.proxy, buf1, buf2)
	noop3.Init(link.proxy, buf2, buf3)
	noop4.Init(link.proxy, buf3, buf4)
	noop5.Init(link.proxy, buf4, dst)

	go noop1.Pipe()
	go noop2.Pipe()
	go noop3.Pipe()
	go noop4.Pipe()
	go noop5.Pipe()
}

func (link *link) Close() {
	link.client.Close()
	link.upstream.Close()
}
