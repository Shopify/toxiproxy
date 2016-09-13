package toxics

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/Shopify/toxiproxy/stream"
)

type HttpToxic struct{}

type HttpToxicState struct {
	Shared bool
}

func (t *HttpToxic) ModifyResponse(resp *http.Response) {
	resp.Header.Set("Location", "https://github.com/Shopify/toxiproxy")
}

func (t *HttpToxic) PipeRequest(stub *ToxicStub) {
	state := stub.State.(*HttpToxicState)
	state.Shared = true
	// TODO
	new(NoopToxic).Pipe(stub)
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
			fmt.Println("Shared:", state.Shared) // This should be true if the shared state is working
			t.ModifyResponse(resp)
			resp.Write(writer)
		}
		buffer.Reset()
	}
}

func (t *HttpToxic) NewState() interface{} {
	return new(HttpToxicState)
}

func init() {
	Register("http", new(HttpToxic))
}
