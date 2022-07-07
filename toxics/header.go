package toxics

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Shopify/toxiproxy/v2/stream"
)

type HeaderToxic struct {
	Mode    string            `json:"mode"`
	Headers map[string]string `json:"headers"`
}

func (t *HeaderToxic) ModifyResponseHeader(resp *http.Response) {
	for key, value := range t.Headers {
		resp.Header.Set(key, value)
	}
}

func (t *HeaderToxic) ModifyRequestHeader(req *http.Request) {
	for key, value := range t.Headers {
		if strings.EqualFold("Host", key) {
			req.Host = value
		} else {
			req.Header.Set(key, value)
		}
	}
}

func (t *HeaderToxic) PrepareRequest(stub *ToxicStub, buffer *bytes.Buffer, reader *stream.ChanReader, writer *stream.ChanWriter) interface{} {
	for {
		tee := io.TeeReader(reader, buffer)
		req, err := http.ReadRequest(bufio.NewReader(tee))

		if err == stream.ErrInterrupted {
			buffer.WriteTo(writer)
		} else if err == io.EOF {
			stub.Close()
		}
		if err != nil {
			fmt.Println(err)
			buffer.WriteTo(writer)
		} else {
			t.ModifyRequestHeader(req)
			fmt.Println("Req headers")
			for k, v := range req.Header {
				fmt.Print(k, ":", v, "\n")
			}
			req.Write(writer)
		}
		buffer.Reset()
	}
}

func (t *HeaderToxic) PrepareResponse(stub *ToxicStub, buffer *bytes.Buffer, reader *stream.ChanReader, writer *stream.ChanWriter) {
	for {
		tee := io.TeeReader(reader, buffer)
		resp, err := http.ReadResponse(bufio.NewReader(tee), nil)

		if err == stream.ErrInterrupted {
			buffer.WriteTo(writer)
		} else if err == io.EOF {
			stub.Close()
		}
		if err != nil {
			buffer.WriteTo(writer)
		} else {
			t.ModifyResponseHeader(resp)
			resp.Write(writer)
		}
		buffer.Reset()
	}
}

func (t *HeaderToxic) Pipe(stub *ToxicStub) {
	buffer := bytes.NewBuffer(make([]byte, 0, 32*1024))
	writer := stream.NewChanWriter(stub.Output)
	reader := stream.NewChanReader(stub.Input)
	reader.SetInterrupt(stub.Interrupt)

	if t.Mode == "response" {
		t.PrepareResponse(stub, buffer, reader, writer)
	} else {
		t.PrepareRequest(stub, buffer, reader, writer)
	}
}

func init() {
	Register("header", new(HeaderToxic))
}
