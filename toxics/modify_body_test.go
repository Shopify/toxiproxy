package toxics_test

import (
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/Shopify/toxiproxy/v2/toxics/httputils"
)

func TestToxicModifiesHTTPResponseBody(t *testing.T) {
	http.HandleFunc("/", echoHelloWorld)

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

	body, _ := ioutil.ReadAll(resp.Body)

	AssertBodyNotEqual(t, body, []byte(httputils.Status400))

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "modify_body", "downstream", &toxics.ModifyBodyToxic{Body: httputils.Status400}))

	resp, err = http.Get("http://" + proxy.Listen)
	if err != nil {
		t.Error("Failed to connect to proxy", err)
	}

	body, _ = ioutil.ReadAll(resp.Body)

	AssertBodyEqual(t, body, []byte(httputils.Status400))

}
