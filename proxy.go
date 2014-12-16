package main

import (
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"gopkg.in/tomb.v1"

	"net"
)

// Proxy represents the proxy in its entirity with all its links. The main
// responsibility of Proxy is to accept new client and create Links between the
// client and upstream.
//
// Client <-> toxiproxy <-> Upstream
//
type Proxy struct {
	sync.Mutex

	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`

	started chan error

	tomb        tomb.Tomb
	connections map[string]net.Conn
	upToxics    *ToxicCollection
	downToxics  *ToxicCollection
}

var ErrProxyAlreadyStarted = errors.New("Proxy already started")

func NewProxy() *Proxy {
	proxy := &Proxy{
		started:     make(chan error),
		connections: make(map[string]net.Conn),
	}
	proxy.upToxics = NewToxicCollection(proxy)
	proxy.downToxics = NewToxicCollection(proxy)
	return proxy
}

func (proxy *Proxy) Start() error {
	proxy.Lock()
	defer proxy.Unlock()

	if proxy.Enabled {
		return ErrProxyAlreadyStarted
	}
	proxy.Enabled = true

	go proxy.server()
	err := <-proxy.started
	// Disable the proxy again if it failed to start
	proxy.Enabled = err == nil
	return err
}

// server runs the Proxy server, accepting new clients and creating Links to
// connect them to upstreams.
func (proxy *Proxy) server() {
	ln, err := net.Listen("tcp", proxy.Listen)
	if err != nil {
		proxy.started <- err
		return
	}

	proxy.Listen = ln.Addr().String()
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

		upstream, err := net.Dial("tcp", proxy.Upstream)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":     proxy.Name,
				"client":   client.RemoteAddr(),
				"proxy":    proxy.Listen,
				"upstream": proxy.Upstream,
			}).Error("Unable to open connection to upstream")
			client.Close()
			continue
		}

		name := client.RemoteAddr().String()
		proxy.Lock()
		proxy.connections[name+"client"] = client
		proxy.connections[name+"upstream"] = upstream
		proxy.Unlock()
		proxy.upToxics.StartLink(name+"client", client, upstream)
		proxy.downToxics.StartLink(name+"upstream", upstream, client)
	}
}

func (proxy *Proxy) RemoveConnection(name string) {
	proxy.Lock()
	defer proxy.Unlock()
	delete(proxy.connections, name)
}

func (proxy *Proxy) Stop() {
	proxy.Lock()
	defer proxy.Unlock()
	if !proxy.Enabled {
		return
	}
	proxy.Enabled = false
	proxy.Unlock()

	proxy.tomb.Killf("Shutting down from stop()")
	proxy.tomb.Wait() // Wait until we stop accepting new connections

	proxy.Lock()
	for _, conn := range proxy.connections {
		conn.Close()
	}

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Terminated proxy")
}
