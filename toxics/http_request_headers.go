package toxics

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/Shopify/toxiproxy/stream"
)

// HttpToxic modifies requests headers (upstream) for http requests. Not to be used with direction = downstream
type HttpToxic struct {
	Headers map[string]string `json:"headers"`
}

func (t *HttpToxic) modifyRequest(request *http.Request) {
	// Add all headers to request. Host is derived from the url if we dont set it explicitly.
	for k, v := range t.Headers {
		if strings.EqualFold("Host", k) {
			request.Host = v
		} else {
			request.Header.Set(k, v)
		}
	}
}

func (t *HttpToxic) Pipe(stub *ToxicStub) {
	buffer := bytes.NewBuffer(make([]byte, 0, 32*1024))
	writer := stream.NewChanWriter(stub.Output)
	reader := stream.NewChanReader(stub.Input)
	reader.SetInterrupt(stub.Interrupt)
	for {
		tee := io.TeeReader(reader, buffer)
		req, err := http.ReadRequest(bufio.NewReader(tee))
		if err == stream.ErrInterrupted {
			buffer.WriteTo(writer)
			return
		} else if err == io.EOF {
			stub.Close()
			return
		}
		if err != nil {
			buffer.WriteTo(writer)
		} else {
			t.modifyRequest(req)
			req.Write(writer)
		}
		buffer.Reset()
	}
}

func init() {
	Register("http_request_headers", new(HttpToxic))
}
