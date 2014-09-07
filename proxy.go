package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/tomb"

	"net"
	"os"
	"time"
)

var AccepTimeout = time.Second

func init() {
	// Lower timeout makes the proxy quit faster
	if os.Getenv("ENV") == "test" {
		AccepTimeout = time.Millisecond
	}
}

// Proxy represents the proxy in its entirity with all its links. The main
// responsibility of Proxy is to accept new client and create Links between the
// client and upstream.
type Proxy struct {
	sync.Mutex

	Name     string
	Listen   string
	Upstream string

	started chan bool

	tomb  tomb.Tomb
	links []*link
}

func NewProxy() *Proxy {
	return &Proxy{
		started: make(chan bool, 1),
	}
}

func (proxy *Proxy) Start() {
	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Starting proxy")

	go proxy.server()
}

// server runs the Proxy server, accepting new clients and creating Links to
// connect them to upstreams.
func (proxy *Proxy) server() {
	ln, err := net.Listen("tcp", proxy.Listen)
	if err != nil {
		logrus.WithFields(logrus.Fields{"upstream": proxy.Upstream, "err": err}).Error("Unable to start proxy server")
		return
	}

	// This is a super hacky way to get a local address correct.
	// We want to set #Listen because if it's not supplied in the API we'll just
	// use an ephemeral port.
	tcpAddr := ln.Addr().(*net.TCPAddr)
	tcpAddrIp := strings.Trim(string(tcpAddr.IP), "\u0000")
	if tcpAddrIp == "" {
		tcpAddrIp = "localhost"
	}
	proxy.Listen = fmt.Sprintf("%s:%d", tcpAddrIp, tcpAddr.Port)

	proxy.started <- true

	for {
		// Set a deadline to not make Accept() block forever, allowing us to shut
		// down this thread.
		err = ln.(*net.TCPListener).SetDeadline(time.Now().Add(AccepTimeout))
		if err != nil {
			logrus.WithField("name", proxy.Name).Fatal("Unable to set deadline")
		}

		// Shut down if the tomb is not empty
		select {
		case <-proxy.tomb.Dying():
			if err := ln.Close(); err != nil {
				logrus.WithFields(logrus.Fields{
					"proxy":    proxy.Listen,
					"upstream": proxy.Upstream,
					"name":     proxy.Name,
					"err":      err,
				}).Warn("Failed to shut down proxy server")
			}
			proxy.tomb.Done()
			return
		default:
		}

		client, err := ln.Accept()
		if err != nil {
			if !err.(*net.OpError).Timeout() {
				logrus.WithFields(logrus.Fields{"proxy": proxy.Listen, "err": err}).Error("Unable to accept client")
			}
			continue
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
