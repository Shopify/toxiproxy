package main

import (
	"io"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
)

// Link is the TCP link between a client and an upstream.
//
// Client <-> toxiproxy <-> Upstream
//
// Its responsibility is to shove from each side to the other. Clients don't
// need to know they are talking to the upsream through toxiproxy.
type link struct {
	sync.Mutex
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
	link.Lock()
	defer link.Unlock()

	link.upstream, err = net.Dial("tcp", link.proxy.Upstream)
	if err != nil {
		return err
	}

	go link.pipe(link.client, link.upstream)
	go link.pipe(link.upstream, link.client)

	return nil
}

func (link *link) pipe(src, dst net.Conn) {
	_, err := io.Copy(dst, src)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"name":     link.proxy.Name,
			"upstream": link.proxy.Upstream,
			"err":      err,
		}).Warn("Client or source terminated")
	}

	link.Lock()
	defer link.Unlock()

	link.client.Close()
	link.upstream.Close()
}

func (link *link) Close() {
	link.Lock()
	defer link.Unlock()

	link.client.Close()
	link.upstream.Close()
}
