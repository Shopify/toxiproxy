package toxiproxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	toxiproxy "github.com/Shopify/toxiproxy/v2/client"
)

func TestClient_Headers(t *testing.T) {
	t.Parallel()

	expected := "toxiproxy-cli/v1.25.0 (darwin/arm64)"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		ua := r.Header.Get("User-Agent")

		if ua != expected {
			t.Errorf("User-Agent for %s %s is expected `%s', got: `%s'",
				r.Method,
				r.URL,
				expected,
				ua)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type for %s %s is expected `application/json', got: `%s'",
				r.Method,
				r.URL,
				contentType)
		}
		w.Write([]byte(`foo`))
	}))
	defer server.Close()

	client := toxiproxy.NewClient(server.URL)
	client.UserAgent = expected

	cases := []struct {
		name string
		fn   func(c *toxiproxy.Client)
	}{
		{"get version", func(c *toxiproxy.Client) { c.Version() }},
		{"get proxies", func(c *toxiproxy.Client) { c.Proxies() }},
		{"create proxy", func(c *toxiproxy.Client) {
			c.CreateProxy("foo", "example.com:0", "example.com:0")
		}},
		{"get proxy", func(c *toxiproxy.Client) { c.Proxy("foo") }},
		{"post populate", func(c *toxiproxy.Client) {
			c.Populate([]toxiproxy.Proxy{{}})
		}},
		{"create toxic", func(c *toxiproxy.Client) {
			c.AddToxic(&toxiproxy.ToxicOptions{})
		}},
		{"update toxic", func(c *toxiproxy.Client) {
			c.UpdateToxic(&toxiproxy.ToxicOptions{})
		}},
		{"delete toxic", func(c *toxiproxy.Client) {
			c.RemoveToxic(&toxiproxy.ToxicOptions{})
		}},
		{"reset state", func(c *toxiproxy.Client) {
			c.ResetState()
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(client)
		})
	}
}
