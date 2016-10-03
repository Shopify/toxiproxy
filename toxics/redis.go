package toxics

import (
	"bufio"
	"fmt"
	"io"

	"github.com/Shopify/toxiproxy/stream"
)

type RedisToxic struct {
	FailWrites bool `json:"fail_writes"`
}

type RedisToxicState struct {
	Command chan stream.RedisType
}

func (t *RedisToxic) PipeUpstream(stub *ToxicStub) {
	state := stub.State.(*RedisToxicState)

	reader := bufio.NewReader(stub.Reader)
	for {
		cmd, err := stream.ParseRESP(reader)
		if stub.HandleReadError(err) {
			if err == io.EOF {
				close(state.Command)
			}
			return
		} else if err == nil {
			state.Command <- cmd
			str := cmd.StringArray()
			fmt.Println("Command:", str)
			if len(str) > 0 && str[0] == "SET" {
				// Skip the backend server
			} else {
				stub.Writer.Write(cmd.Raw())
			}
			stub.Reader.Checkpoint(-reader.Buffered())
		}
	}
}

func (t *RedisToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*RedisToxicState)

	reader := bufio.NewReader(stub.Reader)
	for {
		resp, err := stream.ParseRESP(reader)
		if stub.HandleReadError(err) {
			return
		} else {
			select {
			case cmd := <-state.Command:
				str := cmd.StringArray()
				if len(str) > 0 && str[0] == "SET" {
					stub.Writer.Write(stream.RedisType{stream.Error, "ERR write failure"}.Raw())
				} else {
					stub.Writer.Write(resp.Raw())
				}
			default:
				stub.Writer.Write(resp.Raw())
			}
			stub.Reader.Checkpoint(-reader.Buffered())
		}
	}
}

func (t *RedisToxic) NewState() interface{} {
	return &RedisToxicState{
		Command: make(chan stream.RedisType, 1),
	}
}

func init() {
	Register("redis", new(RedisToxic))
}
