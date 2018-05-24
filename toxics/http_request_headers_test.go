package toxics_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/Shopify/toxiproxy/toxics"
)

func echoRequestHeaders(w http.ResponseWriter, r *http.Request) {
	headersMap := map[string]string{}

	for k, v := range r.Header {
		// headers can contain multiple elements. for the purposes of this test we pick the 1st
		headersMap[k] = v[0]
	}

	mapAsJson, _ := json.Marshal(headersMap)
	w.Write([]byte(mapAsJson))
}

func TestToxicAddsHttpHeaders(t *testing.T) {
	http.HandleFunc("/", echoRequestHeaders)

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create TCP server", err)
	}

	go http.Serve(ln, nil)
	defer ln.Close()

	proxy := NewTestProxy("test", ln.Addr().String())
	proxy.Start()
	defer proxy.Stop()

	resp, err := http.Get("http://" + proxy.Listen)
	if err != nil {
		t.Error("Failed to connect to proxy", err)
	}

	body, err := ioutil.ReadAll(resp.Body)

	AssertDoesNotContainHeader(t, string(body), "Foo", "Bar")
	AssertDoesNotContainHeader(t, string(body), "Lorem", "Ipsum")

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "http_request_headers", "upstream", &toxics.HttpToxic{Headers: map[string]string{"Foo": "Bar", "Lorem": "Ipsum"}}))

	resp, err = http.Get("http://" + proxy.Listen)
	if err != nil {
		t.Error("Failed to connect to proxy", err)
	}

	body, err = ioutil.ReadAll(resp.Body)

	AssertContainsHeader(t, string(body), "Foo", "Bar")
	AssertContainsHeader(t, string(body), "Lorem", "Ipsum")
}

func AssertDoesNotContainHeader(t *testing.T, body string, headerKey string, headerValue string) {
	containsHeader := strings.Contains(string(body), fmt.Sprintf(`"%s":"%s"`, headerKey, headerValue))

	if containsHeader {
		t.Errorf("Unexpected header found. Header=%s", headerKey)
	}
}

func AssertContainsHeader(t *testing.T, body string, headerKey string, headerValue string) {
	containsHeader := strings.Contains(string(body), fmt.Sprintf(`"%s":"%s"`, headerKey, headerValue))

	if !containsHeader {
		t.Errorf("Expected header not found. Header=%s", headerKey)
	}
}
