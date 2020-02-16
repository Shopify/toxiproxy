package toxiproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"sync"
	"time"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/sirupsen/logrus"
	tomb "gopkg.in/tomb.v1"

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

	Name     string   `json:"name"`
	Listen   string   `json:"listen"`
	Upstream string   `json:"upstream"`
	Enabled  bool     `json:"enabled"`
	TLS      *TlsData `json:"tls,omitempty"`

	started chan error

	caCert      *tls.Certificate
	tomb        tomb.Tomb
	connections ConnectionList
	Toxics      *ToxicCollection `json:"-"`
}

type TlsData struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
	// When the cert and key represent a CA, this can is used to dynamically sign fake certificates created with proper CN
	IsCA bool `json:"isCA",omitempty`
	// By default this is false (we are doing MITM attack so why bother with upstream certificate check)
	VerifyUpstream bool `json:"verifyUpstream",omitempty`
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

// Called for each new connection
func (proxy *Proxy) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	var name string

	if hello.ServerName == "" {
		name = "default"
	} else {
		name = hello.ServerName
	}

	logrus.WithFields(logrus.Fields{
		"proxy":      proxy.Name,
		"serverName": name,
	}).Info("getCertificate called")

	if proxy.caCert == nil {
		return nil, errors.New("No CA certificate found")
	}

	// Dynamically create new cert based on SNI
	cert, err := createCertificate(*proxy.caCert, name)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"proxy":      proxy.Name,
			"serverName": name,
		}).Errorf("created returned an error %s", err)

		return nil, err
	}
	return cert, nil
}

// Ensure the given file is a CA certificate
func ensureCaCert(file string) error {
	certFile, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(certFile)

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	if cert.KeyUsage&x509.KeyUsageCertSign != x509.KeyUsageCertSign && !cert.IsCA {
		return errors.New(fmt.Sprintf("The given certificate is not a CA cert - usage %d, isCA %t", cert.KeyUsage, cert.IsCA))
	}

	return nil
}

// Utility function to create new certificate with given common name signed with our CA
func createCertificate(caTls tls.Certificate, commonName string) (*tls.Certificate, error) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1337),
		Subject: pkix.Name{
			Organization: []string{"Toxiproxy"},
			CommonName:   commonName,
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		IsCA:         false,
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &priv.PublicKey

	ca, err := x509.ParseCertificate(caTls.Certificate[0])
	if err != nil {
		return nil, err
	}

	certBlock, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, caTls.PrivateKey)
	if err != nil {
		return nil, err
	}

	newCert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBlock}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}),
	)

	if err != nil {
		return nil, err
	}

	return &newCert, nil
}

// server runs the Proxy server, accepting new clients and creating Links to
// connect them to upstreams.
func (proxy *Proxy) server() {
	var (
		ln       net.Listener
		err      error
		upstream net.Conn
		config   tls.Config
	)

	// Logging
	if proxy.TLS != nil {
		logrus.WithFields(logrus.Fields{
			"proxy":          proxy.Name,
			"cert":           proxy.TLS.Cert,
			"key":            proxy.TLS.Key,
			"isCA":           proxy.TLS.IsCA,
			"verifyUpstream": proxy.TLS.VerifyUpstream,
		}).Info("TLS certificates were specified")

		if proxy.TLS.IsCA {
			err := ensureCaCert(proxy.TLS.Cert)
			if err != nil {
				proxy.started <- err
				return
			}
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"proxy": proxy.Name,
		}).Info("TLS certificates were NOT specified")
	}

	// Action
	if proxy.TLS != nil {
		cert, err := tls.LoadX509KeyPair(proxy.TLS.Cert, proxy.TLS.Key)
		if err != nil {
			proxy.started <- err
			return
		}

		if proxy.TLS.IsCA {
			config = tls.Config{GetCertificate: proxy.getCertificate}
			proxy.caCert = &cert
		} else {
			config = tls.Config{Certificates: []tls.Certificate{cert}}
			proxy.caCert = nil
		}

		config.Rand = rand.Reader

		ln, err = tls.Listen("tcp", proxy.Listen, &config)
	} else {
		ln, err = net.Listen("tcp", proxy.Listen)
	}

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

		if proxy.TLS != nil {
			clientConfig := &tls.Config{InsecureSkipVerify: !proxy.TLS.VerifyUpstream}
			upstreamTLS, err := tls.Dial("tcp", proxy.Upstream, clientConfig)

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
			upstream = upstreamTLS

		} else {
			upstreamPlain, err := net.Dial("tcp", proxy.Upstream)

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

			upstream = upstreamPlain
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

// Starts a proxy, assumes the lock has already been taken
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

// Stops a proxy, assumes the lock has already been taken
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
