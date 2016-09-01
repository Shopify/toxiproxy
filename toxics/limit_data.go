package toxics

import "github.com/Shopify/toxiproxy/stream"

// LimitDataToxic has limit in bytes
type LimitDataToxic struct {
	Bytes            int64 `json:"bytes"`
	bytesTransmitted int64
}

func (t *LimitDataToxic) Pipe(stub *ToxicStub) {
	var bytesRemaining = t.Bytes - t.bytesTransmitted

	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}

			chunk := c

			if bytesRemaining < int64(len(c.Data)) {
				chunk = &stream.StreamChunk{
					Timestamp: c.Timestamp,
					Data:      c.Data[0:bytesRemaining],
				}
			}

			if len(chunk.Data) > 0 {
				stub.Output <- chunk
				t.bytesTransmitted += int64(len(chunk.Data))
			}

			bytesRemaining = t.Bytes - t.bytesTransmitted

			if bytesRemaining <= 0 {
				stub.Close()
				return
			}
		}
	}
}

func init() {
	Register("limit_data", new(LimitDataToxic))
}
