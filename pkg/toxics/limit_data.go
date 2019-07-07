package toxics

import (
	"github.com/Shopify/toxiproxy/pkg/stream"
)

// LimitData has limit in bytes
type LimitData struct {
	Bytes int64 `json:"bytes"`
}

type LimitDataState struct {
	bytesTransmitted int64
}

func (t *LimitData) Pipe(stub *Stub) {
	state := stub.State.(*LimitDataState)
	bytesRemaining := t.Bytes - state.bytesTransmitted

	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}

			if bytesRemaining < 0 {
				bytesRemaining = 0
			}

			if bytesRemaining < int64(len(c.Data)) {
				c = &stream.Chunk{
					Timestamp: c.Timestamp,
					Data:      c.Data[0:bytesRemaining],
				}
			}

			if len(c.Data) > 0 {
				stub.Output <- c
				state.bytesTransmitted += int64(len(c.Data))
			}

			bytesRemaining = t.Bytes - state.bytesTransmitted

			if bytesRemaining <= 0 {
				stub.Close()
				return
			}
		}
	}
}

func (t *LimitData) NewState() interface{} {
	return new(LimitDataState)
}

func init() {
	Register("limit_data", new(LimitData))
}
