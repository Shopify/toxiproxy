package toxiproxy

import (
	"errors"
	"net"
	"sync"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/sirupsen/logrus"
	tomb "gopkg.in/tomb.v1"
)

// Proxy represents the proxy in its entirety with all its links. The main
// responsibility of Proxy is to accept new client and create Links between the
// client and upstream.
//
// Client <-> toxiproxy <-> Upstream.
//
type Proxy struct {
	sync.Mutex

	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`

	listener net.Listener
	started  chan error

	tomb        tomb.Tomb
	connections ConnectionList
	Toxics      *ToxicCollection `json:"-"`
}

type ConnectionList struct {
	list map[string]net.Conn
	lock sync.Mutex
}

func (c *ConnectionList) Lock() {
	c.lock.Lock()
}

func (c *ConnectionList) Unlock() {
	c.lock.Unlock()
}

var ErrProxyAlreadyStarted = errors.New("Proxy already started")

func NewProxy() *Proxy {
	proxy := &Proxy{
		started:     make(chan error),
		connections: ConnectionList{list: make(map[string]net.Conn)},
	}
	proxy.Toxics = NewToxicCollection(proxy)
	return proxy
}

func (proxy *Proxy) Start() error {
	proxy.Lock()
	defer proxy.Unlock()

	return start(proxy)
}

func (proxy *Proxy) Update(input *Proxy) error {
	proxy.Lock()
	defer proxy.Unlock()

	if input.Listen != proxy.Listen || input.Upstream != proxy.Upstream {
		stop(proxy)
		proxy.Listen = input.Listen
		proxy.Upstream = input.Upstream
	}

	if input.Enabled != proxy.Enabled {
		if input.Enabled {
			return start(proxy)
		}
		stop(proxy)
	}
	return nil
}

func (proxy *Proxy) Stop() {
	proxy.Lock()
	defer proxy.Unlock()

	stop(proxy)
}

func (proxy *Proxy) listen() error {
	var err error
	proxy.listener, err = net.Listen("tcp", proxy.Listen)
	if err != nil {
		proxy.started <- err
		return err
	}
	proxy.Listen = proxy.listener.Addr().String()
	proxy.started <- nil

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Started proxy")

	return nil
}

func (proxy *Proxy) close() {
	// Unblock proxy.listener.Accept()
	err := proxy.listener.Close()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"proxy":  proxy.Name,
			"listen": proxy.Listen,
			"err":    err,
		}).Warn("Attempted to close an already closed proxy server")
	}
}

// This channel is to kill the blocking Accept() call below by closing the
// net.Listener.
func (proxy *Proxy) freeBlocker(acceptTomb *tomb.Tomb) {
	<-proxy.tomb.Dying()

	// Notify ln.Accept() that the shutdown was safe
	acceptTomb.Killf("Shutting down from stop()")

	proxy.close()

	// Wait for the accept loop to finish processing
	acceptTomb.Wait()
	proxy.tomb.Done()
}

// server runs the Proxy server, accepting new clients and creating Links to
// connect them to upstreams.
func (proxy *Proxy) server() {
	err := proxy.listen()
	if err != nil {
		return
	}

	acceptTomb := &tomb.Tomb{}
	defer acceptTomb.Done()

	// This channel is to kill the blocking Accept() call below by closing the
	// net.Listener.
	go proxy.freeBlocker(acceptTomb)

	for {
		client, err := proxy.listener.Accept()
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
			}).Error("Unable to open connection to upstream: " + err.Error())
			client.Close()
			continue
		}

		name := client.RemoteAddr().String()
		proxy.connections.Lock()
		proxy.connections.list[name+"upstream"] = upstream
		proxy.connections.list[name+"downstream"] = client
		proxy.connections.Unlock()
		proxy.Toxics.StartLink(name+"upstream", client, upstream, stream.Upstream)
		proxy.Toxics.StartLink(name+"downstream", upstream, client, stream.Downstream)
	}
}

func (proxy *Proxy) RemoveConnection(name string) {
	proxy.connections.Lock()
	defer proxy.connections.Unlock()
	delete(proxy.connections.list, name)
}

// Starts a proxy, assumes the lock has already been taken.
func start(proxy *Proxy) error {
	if proxy.Enabled {
		return ErrProxyAlreadyStarted
	}

	proxy.tomb = tomb.Tomb{} // Reset tomb, from previous starts/stops
	go proxy.server()
	err := <-proxy.started
	// Only enable the proxy if it successfully started
	proxy.Enabled = err == nil
	return err
}

// Stops a proxy, assumes the lock has already been taken.
func stop(proxy *Proxy) {
	if !proxy.Enabled {
		return
	}
	proxy.Enabled = false

	proxy.tomb.Killf("Shutting down from stop()")
	proxy.tomb.Wait() // Wait until we stop accepting new connections

	proxy.connections.Lock()
	defer proxy.connections.Unlock()
	for _, conn := range proxy.connections.list {
		conn.Close()
	}

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Terminated proxy")
}
