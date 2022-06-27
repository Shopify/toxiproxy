package httputils

import "net/http"

// SetHttpStatusCode sets the status code of the response
func SetHttpStatusCode(r *http.Response, statusCode int) {
	switch statusCode {
	case 200:
		r.StatusCode = 200
		r.Status = "200 OK"
	case 201:
		r.StatusCode = 201
		r.Status = "201 Created"
	case 202:
		r.StatusCode = 202
		r.Status = "202 Accepted"
	case 204:
		r.StatusCode = 204
		r.Status = "204 No Content"
	case 301:
		r.StatusCode = 301
		r.Status = "301 Moved Permanently"
	case 302:
		r.StatusCode = 302
		r.Status = "302 Found"
	case 304:
		r.StatusCode = 304
		r.Status = "304 Not Modified"
	case 400:
		r.StatusCode = 400
		r.Status = "400 Bad Request"
	case 401:
		r.StatusCode = 401
		r.Status = "401 Unauthorized"
	case 403:
		r.StatusCode = 403
		r.Status = "403 Forbidden"
	case 404:
		r.StatusCode = 404
		r.Status = "404 Not Found"
	case 500:
		r.StatusCode = 500
		r.Status = "500 Internal Server Error"
	case 501:
		r.StatusCode = 501
		r.Status = "501 Not Implemented"
	case 502:
		r.StatusCode = 502
		r.Status = "502 Bad Gateway"
	case 503:
		r.StatusCode = 503
		r.Status = "503 Service Unavailable"
	case 504:
		r.StatusCode = 504
		r.Status = "504 Gateway Timeout"
	default:
		// if the status code is not recognized, do not change it
	}
}
