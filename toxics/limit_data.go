package toxics

import "github.com/Shopify/toxiproxy/stream"

// LimitDataToxic has limit in bytes
type LimitDataToxic struct {
	Bytes int64 `json:"bytes"`
}

type LimitDataToxicState struct {
	bytesTransmitted int64
}

func (t *LimitDataToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*LimitDataToxicState)
	var bytesRemaining = t.Bytes - state.bytesTransmitted

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
				c = &stream.StreamChunk{
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

func (t *LimitDataToxic) NewState() interface{} {
	return new(LimitDataToxicState)
}

func init() {
	Register("limit_data", new(LimitDataToxic))
}
