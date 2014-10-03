package main

import (
	"io"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
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

	go link.pipe(link.client, link.upstream)
	go link.pipe(link.upstream, link.client)

	return nil
}

func (link *link) pipe(src, dst net.Conn) {
	pipe := NewPipe(link.proxy, src)
	pipe2 := NewPipe(link.proxy, pipe)

	latency := new(LatencyToxic)
	latency.Latency = time.Millisecond * 300
	pipe.Start(latency)
	pipe2.Start(new(NoopToxic))

	bytes, err := io.Copy(dst, pipe2)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"name":     link.proxy.Name,
			"upstream": link.proxy.Upstream,
			"bytes":    bytes,
			"err":      err,
		}).Warn("Client or source terminated")
	}

	link.Close()
}

func (link *link) Close() {
	link.client.Close()
	link.upstream.Close()
}
