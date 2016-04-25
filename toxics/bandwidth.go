package toxics

import (
	"time"

	"github.com/Shopify/toxiproxy/stream"
)

// The BandwidthToxic passes data through at a limited rate
type BandwidthToxic struct {
	// Rate in KB/s
	Rate int64 `json:"rate"`
}

func (t *BandwidthToxic) Pipe(stub *ToxicStub) {
	var sleep time.Duration = 0
	for {
		select {
		case <-stub.Interrupt:
			return
		case p := <-stub.Input:
			if p == nil {
				stub.Close()
				return
			}
			if t.Rate <= 0 {
				sleep = 0
			} else {
				sleep += time.Duration(len(p.Data)) * time.Millisecond / time.Duration(t.Rate)
			}
			// If the rate is low enough, split the packet up and send in 100 millisecond intervals
			for int64(len(p.Data)) > t.Rate*100 {
				select {
				case <-time.After(100 * time.Millisecond):
					stub.Output <- &stream.StreamChunk{p.Data[:t.Rate*100], p.Timestamp}
					p.Data = p.Data[t.Rate*100:]
					sleep -= 100 * time.Millisecond
				case <-stub.Interrupt:
					stub.Output <- p // Don't drop any data on the floor
					return
				}
			}
			start := time.Now()
			select {
			case <-time.After(sleep):
				// time.After only seems to have ~1ms prevision, so offset the next sleep by the error
				sleep -= time.Since(start)
				stub.Output <- p
			case <-stub.Interrupt:
				stub.Output <- p // Don't drop any data on the floor
				return
			}
		}
	}
}

func init() {
	Register("bandwidth", new(BandwidthToxic))
}
