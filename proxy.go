package main

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/tomb"

	"net"
)

// Proxy represents the proxy in its entirity with all its links. The main
// responsibility of Proxy is to accept new client and create Links between the
// client and upstream.
type Proxy struct {
	sync.Mutex

	Name     string
	Listen   string
	Upstream string

	started chan error

	tomb  tomb.Tomb
	links []*link
}

func NewProxy() *Proxy {
	proxy := &Proxy{}
	proxy.allocate()
	return proxy
}

// allocate instantiates the necessary dependencies. This is in a seperate
// method because it allows us to read Proxies from JSON and then call
// `allocate()` on them, sharing this with `NewProxy()`.
func (proxy *Proxy) allocate() {
	proxy.started = make(chan error)
}

func (proxy *Proxy) Start() error {
	go proxy.server()
	return <-proxy.started
}

// server runs the Proxy server, accepting new clients and creating Links to
// connect them to upstreams.
func (proxy *Proxy) server() {
	ln, err := net.Listen("tcp", proxy.Listen)
	if err != nil {
		proxy.started <- err
		return
	}

	// This is a super hacky way to get a local address correct.
	// We want to set #Listen because if it's not supplied in the API we'll just
	// use an ephemeral port.
	tcpAddr := ln.Addr().(*net.TCPAddr)
	tcpAddrIp := string(tcpAddr.IP)
	if net.ParseIP(string(tcpAddr.IP)) == nil {
		tcpAddrIp = "127.0.0.1"
	}
	proxy.Listen = fmt.Sprintf("%s:%d", tcpAddrIp, tcpAddr.Port)

	proxy.started <- nil

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Started proxy")

	quit := make(chan bool)

	// This channel is to kill the blocking Accept() call below by closing the
	// net.Listener.
	go func() {
		<-proxy.tomb.Dying()

		err := ln.Close()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"proxy":  proxy.Name,
				"listen": proxy.Listen,
			}).Warn("Attempted to close an already closed proxy server")
		}

		quit <- true

		proxy.tomb.Done()
	}()

	for {
		client, err := ln.Accept()
		if err != nil {
			// This is to confirm we're being shut down in a legit way. Unfortunately,
			// Go doesn't export the error when it's closed from Close() so we have to
			// sync up with a channel here.
			//
			// See http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-quit:
			default:
				logrus.WithFields(logrus.Fields{
					"proxy":  proxy.Name,
					"listen": proxy.Listen,
					"err":    err,
				}).Warn("Error while accepting client")
			}
			return
		}

		logrus.WithFields(logrus.Fields{
			"name":     proxy.Name,
			"client":   client.RemoteAddr(),
			"proxy":    proxy.Listen,
			"upstream": proxy.Upstream,
		}).Info("Accepted client")

		proxy.Lock()
		link := NewLink(proxy, client)
		proxy.links = append(proxy.links, link)
		proxy.Unlock()

		if err := link.Open(); err != nil {
			logrus.WithFields(logrus.Fields{
				"name":     proxy.Name,
				"client":   client.RemoteAddr(),
				"proxy":    proxy.Listen,
				"upstream": proxy.Upstream,
			}).Error("Unable to open connection to upstream")
		}
	}
}

func (proxy *Proxy) Stop() {
	proxy.tomb.Killf("Shutting down from stop()")

	proxy.Lock()
	for _, link := range proxy.links {
		link.Close()
	}
	proxy.Unlock()

	proxy.tomb.Wait()

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Terminated proxy")
}
