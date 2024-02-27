package matchers

import (
	"bufio"
	"bytes"
	"net/http"
	"regexp"
)

type HttpRequestHeaderMatcher struct {
	HeaderKey        string `json:"headerKey"`
	HeaderValueRegex string `json:"headerValueRegex"`
}

func (m *HttpRequestHeaderMatcher) TryMatch(data []byte) (bool, error) {
	bufioReader := bufio.NewReader(bytes.NewReader(data))

	// Try to parse the data as a HTTP request.
	req, err := http.ReadRequest(bufioReader)
	if err != nil {
		return false, err
	}

	// Try to match the header using the regex.
	headerValue := req.Header.Get(m.HeaderKey)
	match, err := regexp.MatchString(m.HeaderValueRegex, headerValue)
	if err != nil {
		return false, err
	}

	return match, nil
}
