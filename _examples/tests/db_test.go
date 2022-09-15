package main

import (
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	toxiServer "github.com/Shopify/toxiproxy/v2"
	toxiproxy "github.com/Shopify/toxiproxy/v2/client"
	pg "github.com/go-pg/pg/v10"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var db *pg.DB
var toxi *toxiproxy.Client
var proxies map[string]*toxiproxy.Proxy

func DB() *pg.DB {
	if db == nil {
		var err error
		db, err = setupDB(":35432", "sample_test")
		if err != nil {
			log.Panicf("Could not connect to DB: %+v", err)
		}
	}
	return db
}

func connectDB(addr string) *pg.DB {
	return pg.Connect(&pg.Options{
		Addr:     addr,
		User:     "postgres",
		Database: "sample_test",
	})
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("=== SETUP")
	runToxiproxyServer()
	populateProxies()
}

func populateProxies() {
	if toxi == nil {
		toxi = toxiproxy.NewClient("localhost:8474")
	}

	var err error
	_, err = toxi.Populate([]toxiproxy.Proxy{{
		Name:     "postgresql",
		Listen:   "localhost:35432",
		Upstream: "localhost:5432",
		Enabled:  true,
	}})
	if err != nil {
		panic(err)
	}

	proxies, err = toxi.Proxies()
	if err != nil {
		panic(err)
	}
}

func runToxiproxyServer() {
	var err error
	timeout := 5 * time.Second

	// Check if there is instance run
	conn, err := net.DialTimeout("tcp", "localhost:8474", timeout)
	if err == nil {
		conn.Close()
		return
	}

	go func() {
		metricsContainer := toxiServer.NewMetricsContainer(prometheus.NewRegistry())
		server := toxiServer.NewServer(metricsContainer, zerolog.Nop())
		server.Listen("localhost:8474")
	}()

	for i := 0; i < 10; i += 1 {
		conn, err := net.DialTimeout("tcp", "localhost:8474", timeout)
		if err == nil {
			conn.Close()
			return
		}
	}
	panic(err)
}

func TestSlowDBConnection(t *testing.T) {
	db := DB()

	// Add 1s latency to 100% of downstream connections
	proxies["postgresql"].AddToxic("latency_down", "latency", "downstream", 1.0, toxiproxy.Attributes{
		"latency": 10000,
	})
	defer proxies["postgresql"].RemoveToxic("latency_down")

	err := process(db)
	if err != nil {
		t.Fatalf("got error %v, wanted no errors", err)
	}
}

func TestOutageResetPeer(t *testing.T) {
	db := DB()

	// Add broken TCP connection
	proxies["postgresql"].AddToxic("reset_peer_down", "reset_peer", "downstream", 1.0, toxiproxy.Attributes{
		"timeout": 10,
	})
	defer proxies["postgresql"].RemoveToxic("reset_peer_down")

	err := process(db)
	if err == nil {
		t.Fatalf("expect error")
	}
}
