package toxics_test

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/Shopify/toxiproxy/v2/toxics/httputils"
)

func echoHelloWorld(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello World"))
}

func TestToxicModifiesHTTPStatusCode(t *testing.T) {
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

	AssertStatusCodeNotEqual(t, resp.StatusCode, 500)

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "status_code", "downstream", &toxics.StatusCodeToxic{StatusCode: 500, ModifyResponseBody: false}))

	resp, err = http.Get("http://" + proxy.Listen)
	if err != nil {
		t.Error("Failed to connect to proxy", err)
	}
	AssertStatusCodeEqual(t, resp.StatusCode, 500)
}

func TestToxicModifiesBodyWithStatusCode(t *testing.T) {
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

	AssertStatusCodeNotEqual(t, resp.StatusCode, 500)
	AssertBodyNotEqual(t, body, []byte(httputils.Status500))

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "status_code", "downstream", &toxics.StatusCodeToxic{StatusCode: 500, ModifyResponseBody: true}))

	resp, err = http.Get("http://" + proxy.Listen)
	if err != nil {
		t.Error("Failed to connect to proxy", err)
	}

	body, _ = ioutil.ReadAll(resp.Body)

	AssertStatusCodeEqual(t, resp.StatusCode, 500)
	AssertBodyEqual(t, body, []byte(httputils.Status500))

}

func TestUnsupportedStatusCode(t *testing.T) {
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

	statusCode := resp.StatusCode
	initialBody, _ := ioutil.ReadAll(resp.Body)

	proxy.Toxics.AddToxicJson(ToxicToJson(t, "", "status_code", "downstream", &toxics.StatusCodeToxic{StatusCode: 1000, ModifyResponseBody: true}))

	resp, err = http.Get("http://" + proxy.Listen)
	if err != nil {
		t.Error("Failed to connect to proxy", err)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	AssertStatusCodeEqual(t, resp.StatusCode, statusCode)
	AssertBodyEqual(t, body, initialBody)

}

func AssertStatusCodeEqual(t *testing.T, respStatusCode, expectedStatusCode int) {
	if respStatusCode != expectedStatusCode {
		t.Errorf("Response status code {%v} not equal to expected status code {%v}.", respStatusCode, expectedStatusCode)
	}
}

func AssertStatusCodeNotEqual(t *testing.T, respStatusCode, expectedStatusCode int) {
	if respStatusCode == expectedStatusCode {
		t.Errorf("Response status code {%v} equal to expected status code {%v}.", respStatusCode, expectedStatusCode)
	}
}

func AssertBodyEqual(t *testing.T, respBody, expectedBody []byte) {
	if !bytes.Equal(respBody, expectedBody) {
		t.Errorf("Response body {%v} not equal to expected body {%v}.", string(respBody), string(expectedBody))
	}
}

func AssertBodyNotEqual(t *testing.T, respBody, expectedBody []byte) {
	if bytes.Equal(respBody, expectedBody) {
		t.Errorf("Response body {%v} equal to expected body {%v}.", string(respBody), string(expectedBody))
	}
}
