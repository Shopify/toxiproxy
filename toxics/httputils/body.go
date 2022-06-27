package httputils

import (
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	// 3xx pages
	status301 = "<html><head><title>301 Moved Permanently</title></head><body bgcolor='white'><center><h1>301 Moved Permanently</h1></body></html>"
	status302 = "<html><head><title>302 Found</title></head><body bgcolor='white'><center><h1>302 Found</h1></body></html>"
	status304 = "<html><head><title>304 Not Modified</title></head><body bgcolor='white'><center><h1>304 Not Modified</h1></body></html>"
	// 4xx error pages
	status400 = "<html><head><title>400 Bad Request</title></head><body bgcolor='white'><center><h1>400 Bad Request</h1><hr></body></html>"
	status401 = "<html><head><title>401 Unauthorized</title></head><body bgcolor='white'><center><h1>401 Unauthorized</h1><hr></body></html>"
	status403 = "<html><head><title>403 Forbidden</title></head><body bgcolor='white'><center><h1>403 Forbidden</h1><hr></body></html>"
	status404 = "<html><head><title>404 Not Found</title></head><body bgcolor='white'><center><h1>404 Not Found</h1><hr></body></html>"
	// 5xx error pages
	status500 = "<html><head><title>500 Internal Server Error</title></head><body bgcolor='white'><center><h1>500 Internal Server Error</h1></body></html>"
	status501 = "<html><head><title>501 Not Implemented</title></head><body bgcolor='white'><center><h1>501 Not Implemented</h1></body></html>"
	status502 = "<html><head><title>502 Bad Gateway</title></head><body bgcolor='white'><center><h1>502 Bad Gateway</h1></body></html>"
	status503 = "<html><head><title>503 Service Unavailable</title></head><body bgcolor='white'><center><h1>503 Service Unavailable</h1></body></html>"
	status504 = "<html><head><title>504 Gateway Timeout</title></head><body bgcolor='white'><center><h1>504 Gateway Timeout</h1></body></html>"
)

func SetErrorResponseBody(r *http.Response, statusCode int) {
	switch statusCode {
	case 301:
		SetResponseBody(r, status301)
	case 302:
		SetResponseBody(r, status302)
	case 304:
		SetResponseBody(r, status304)
	case 400:
		SetResponseBody(r, status400)
	case 401:
		SetResponseBody(r, status401)
	case 403:
		SetResponseBody(r, status403)
	case 404:
		SetResponseBody(r, status404)
	case 500:
		SetResponseBody(r, status500)
	case 501:
		SetResponseBody(r, status501)
	case 502:
		SetResponseBody(r, status502)
	case 503:
		SetResponseBody(r, status503)
	case 504:
		SetResponseBody(r, status504)
	default:
	}
}

func SetResponseBody(r *http.Response, body string) {
	r.Body = ioutil.NopCloser(strings.NewReader(body))
	r.ContentLength = int64(len(body))
}
