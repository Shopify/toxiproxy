package httputils

import (
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	// 2xx pages
	Status200 = "<html><head><title>200 Status OK</title></head><body bgcolor='white'><center><h1>200 Status OK</h1></body></html>"
	Status201 = "<html><head><title>201 Created</title></head><body bgcolor='white'><center><h1>301 Moved Permanently</h1></body></html>"
	Status202 = "<html><head><title>202 Accepted</title></head><body bgcolor='white'><center><h1>202 Accepted</h1></body></html>"
	Status204 = "<html><head><title>204 No Content</title></head><body bgcolor='white'><center><h1>204 No Content</h1></body></html>"
	// 3xx pages
	Status301 = "<html><head><title>301 Moved Permanently</title></head><body bgcolor='white'><center><h1>301 Moved Permanently</h1></body></html>"
	Status302 = "<html><head><title>302 Found</title></head><body bgcolor='white'><center><h1>302 Found</h1></body></html>"
	Status304 = "<html><head><title>304 Not Modified</title></head><body bgcolor='white'><center><h1>304 Not Modified</h1></body></html>"
	// 4xx error pages
	Status400 = "<html><head><title>400 Bad Request</title></head><body bgcolor='white'><center><h1>400 Bad Request</h1><hr></body></html>"
	Status401 = "<html><head><title>401 Unauthorized</title></head><body bgcolor='white'><center><h1>401 Unauthorized</h1><hr></body></html>"
	Status403 = "<html><head><title>403 Forbidden</title></head><body bgcolor='white'><center><h1>403 Forbidden</h1><hr></body></html>"
	Status404 = "<html><head><title>404 Not Found</title></head><body bgcolor='white'><center><h1>404 Not Found</h1><hr></body></html>"
	// 5xx error pages
	Status500 = "<html><head><title>500 Internal Server Error</title></head><body bgcolor='white'><center><h1>500 Internal Server Error</h1></body></html>"
	Status501 = "<html><head><title>501 Not Implemented</title></head><body bgcolor='white'><center><h1>501 Not Implemented</h1></body></html>"
	Status502 = "<html><head><title>502 Bad Gateway</title></head><body bgcolor='white'><center><h1>502 Bad Gateway</h1></body></html>"
	Status503 = "<html><head><title>503 Service Unavailable</title></head><body bgcolor='white'><center><h1>503 Service Unavailable</h1></body></html>"
	Status504 = "<html><head><title>504 Gateway Timeout</title></head><body bgcolor='white'><center><h1>504 Gateway Timeout</h1></body></html>"
)

func SetErrorResponseBody(r *http.Response, statusCode int) {
	switch statusCode {
	case 200:
		SetResponseBody(r, Status200)
	case 201:
		SetResponseBody(r, Status201)
	case 202:
		SetResponseBody(r, Status202)
	case 204:
		SetResponseBody(r, Status204)
	case 301:
		SetResponseBody(r, Status301)
	case 302:
		SetResponseBody(r, Status302)
	case 304:
		SetResponseBody(r, Status304)
	case 400:
		SetResponseBody(r, Status400)
	case 401:
		SetResponseBody(r, Status401)
	case 403:
		SetResponseBody(r, Status403)
	case 404:
		SetResponseBody(r, Status404)
	case 500:
		SetResponseBody(r, Status500)
	case 501:
		SetResponseBody(r, Status501)
	case 502:
		SetResponseBody(r, Status502)
	case 503:
		SetResponseBody(r, Status503)
	case 504:
		SetResponseBody(r, Status504)
	default:
	}
}

func SetResponseBody(r *http.Response, body string) {
	r.Body = ioutil.NopCloser(strings.NewReader(body))
	r.ContentLength = int64(len(body))
}
