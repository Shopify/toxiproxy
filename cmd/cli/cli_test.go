package main

import (
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"

	toxiproxyServer "github.com/Shopify/toxiproxy/v2"
	toxiproxy "github.com/Shopify/toxiproxy/v2/client"
)

func withAPIServer(t *testing.T, f func(addr string)) {
	t.Helper()

	server := toxiproxyServer.NewServer(
		toxiproxyServer.NewMetricsContainer(prometheus.NewRegistry()),
		zerolog.Nop(),
	)
	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()
	defer func() {
		if err := server.Collection.Clear(); err != nil {
			t.Errorf("Failed to clear collection: %v", err)
		}
	}()

	f(httpServer.URL)
}

func runCLI(t *testing.T, args ...string) error {
	t.Helper()

	app := newApp()
	// Keep cli.Exit errors from terminating the test binary.
	app.ExitErrHandler = func(*cli.Context, error) {}

	return app.Run(append([]string{"toxiproxy-cli"}, args...))
}

func TestCreateProxy(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		enabled bool
	}{
		{"enabled by default", nil, true},
		{"disabled", []string{"--disabled"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withAPIServer(t, func(addr string) {
				args := []string{"--host", addr, "create", "-l", "127.0.0.1:0", "-u", "127.0.0.1:1"}
				args = append(args, tc.args...)
				args = append(args, "mysql_master")

				if err := runCLI(t, args...); err != nil {
					t.Fatalf("create failed: %v", err)
				}

				proxy, err := toxiproxy.NewClient(addr).Proxy("mysql_master")
				if err != nil {
					t.Fatalf("Failed to retrieve proxy: %v", err)
				}

				if proxy.Enabled != tc.enabled {
					t.Errorf("Expected proxy enabled to be %t, got %t", tc.enabled, proxy.Enabled)
				}
				// The server only resolves the listen address once the proxy is started.
				if started := proxy.Listen != "127.0.0.1:0"; started != tc.enabled {
					t.Errorf("Expected proxy started to be %t, got listen %s", tc.enabled, proxy.Listen)
				}
			})
		})
	}
}
