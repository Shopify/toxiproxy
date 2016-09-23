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

	reader := bufio.NewReader(stub.ReadWriter)
	for {
		cmd, err := stream.ParseRESP(reader)
		if stub.ReadWriter.HandleError(err) {
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
				stub.ReadWriter.Checkpoint()
			} else {
				stub.ReadWriter.Write(cmd.Raw())
			}
		}
	}
}

func (t *RedisToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*RedisToxicState)

	reader := bufio.NewReader(stub.ReadWriter)
	for {
		resp, err := stream.ParseRESP(reader)
		if stub.ReadWriter.HandleError(err) {
			return
		} else {
			select {
			case <-stub.Interrupt:
				return
			case cmd := <-state.Command:
				str := cmd.StringArray()
				if len(str) > 0 && str[0] == "SET" {
					stub.ReadWriter.Write(stream.RedisType{stream.Error, "ERR write failure"}.Raw())
				} else {
					stub.ReadWriter.Write(resp.Raw())
				}
			default:
			}
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
