package toxics

import (
	"bufio"
	"bytes"
	"io"
	"net/http"

	"github.com/Shopify/toxiproxy/stream"
)

type HttpToxic struct{}

type HttpToxicState struct {
	Requests chan *http.Request
}

func (t *HttpToxic) FilterRequests(req *http.Request) bool {
	return req.URL.Path == "/foo"
}

func (t *HttpToxic) ModifyResponse(resp *http.Response) {
	resp.Header.Set("Location", "https://github.com/Shopify/toxiproxy")
}

func (t *HttpToxic) PipeRequest(stub *ToxicStub) {
	state := stub.State.(*HttpToxicState)

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
		} else if err == io.EOF || err == io.ErrUnexpectedEOF {
			stub.Close()
			return
		}
		if err != nil {
			buffer.WriteTo(writer)
		} else {
			state.Requests <- req
			req.Write(writer)
		}
		buffer.Reset()
	}
}

func (t *HttpToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*HttpToxicState)

	buffer := bytes.NewBuffer(make([]byte, 0, 32*1024))
	writer := stream.NewChanWriter(stub.Output)
	reader := stream.NewChanReader(stub.Input)
	reader.SetInterrupt(stub.Interrupt)
	for {
		tee := io.TeeReader(reader, buffer)
		resp, err := http.ReadResponse(bufio.NewReader(tee), nil)
		req := <-state.Requests
		if err == stream.ErrInterrupted {
			buffer.WriteTo(writer)
			return
		} else if err == io.EOF || err == io.ErrUnexpectedEOF {
			stub.Close()
			return
		}
		if err != nil {
			buffer.WriteTo(writer)
		} else {
			if t.FilterRequests(req) {
				t.ModifyResponse(resp)
			}
			resp.Write(writer)
		}
		buffer.Reset()
	}
}

func (t *HttpToxic) NewState() interface{} {
	return &HttpToxicState{
		Requests: make(chan *http.Request, 1),
	}
}

func init() {
	Register("http", new(HttpToxic))
}
