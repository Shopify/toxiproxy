package toxiproxy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

func TestWarnIfUnauthenticatedAndExposed(t *testing.T) {
	testCases := []struct {
		name      string
		addr      string
		authToken string
		warns     bool
	}{
		{"localhost", "localhost:8474", "", false},
		{"ipv4 loopback", "127.0.0.1:8474", "", false},
		{"ipv6 loopback", "[::1]:8474", "", false},
		{"all interfaces", "0.0.0.0:8474", "", true},
		{"empty host", ":8474", "", true},
		{"hostname", "toxiproxy.internal:8474", "", true},
		{"all interfaces with token", "0.0.0.0:8474", "secret", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			server := NewServer(NewMetricsContainer(prometheus.NewRegistry()), zerolog.New(&buf))
			server.AuthToken = tc.authToken

			server.warnIfUnauthenticatedAndExposed(tc.addr)

			warned := strings.Contains(buf.String(), "unauthenticated")
			if warned != tc.warns {
				t.Errorf("got warning %t; expected %t (log: %q)", warned, tc.warns, buf.String())
			}
		})
	}
}
