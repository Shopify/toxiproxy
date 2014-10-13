package main

import (
	"github.com/Sirupsen/logrus"
	"gopkg.in/tomb.v1"

	"net"
)

// Proxy represents the proxy in its entirity with all its links. The main
// responsibility of Proxy is to accept new client and create Links between the
// client and upstream.
type Proxy struct {
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

	proxy.started <- nil

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Started proxy")

	acceptTomb := tomb.Tomb{}
	defer acceptTomb.Done()

	// This channel is to kill the blocking Accept() call below by closing the
	// net.Listener.
	go func() {
		<-proxy.tomb.Dying()

		// Notify ln.Accept() that the shutdown was safe
		acceptTomb.Killf("Shutting down from stop()")
		// Unblock ln.Accept()
		err := ln.Close()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"proxy":  proxy.Name,
				"listen": proxy.Listen,
				"err":    err,
			}).Warn("Attempted to close an already closed proxy server")
		}

		// Wait for the accept loop to finish processing
		acceptTomb.Wait()
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
			case <-acceptTomb.Dying():
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

		link := NewLink(proxy, client)
		proxy.links = append(proxy.links, link)

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
	proxy.tomb.Wait() // Wait until we stop accepting new connections

	for _, link := range proxy.links {
		link.Close()
	}

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Terminated proxy")
}
