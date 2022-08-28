package toxics

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Shopify/toxiproxy/v2/stream"
)

// The BandwidthToxic passes data through at a limited rate.
type BandwidthToxic struct {
	// Rate in KB/s
	Rate int64 `json:"rate"`
}

func (t *BandwidthToxic) Pipe(stub *ToxicStub) {
	logger := log.With().
		Str("component", "BandwidthToxic").
		Str("method", "Pipe").
		Str("toxic_type", "bandwidth").
		Str("addr", fmt.Sprintf("%p", t)).
		Logger()
	var sleep time.Duration = 0
	for {
		select {
		case <-stub.Interrupt:
			logger.Trace().Msg("BandwidthToxic was interrupted")
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
					stub.Output <- &stream.StreamChunk{
						Data:      p.Data[:t.Rate*100],
						Timestamp: p.Timestamp,
					}
					p.Data = p.Data[t.Rate*100:]
					sleep -= 100 * time.Millisecond
				case <-stub.Interrupt:
					logger.Trace().Msg("BandwidthToxic was interrupted during writing data")
					err := stub.WriteOutput(p, 5*time.Second) // Don't drop any data on the floor
					if err != nil {
						logger.Warn().Err(err).
							Msg("Could not write last packets after interrupt to Output")
					}
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
				logger.Trace().Msg("BandwidthToxic was interrupted during writing data")
				err := stub.WriteOutput(p, 5*time.Second) // Don't drop any data on the floor
				if err != nil {
					logger.Warn().Err(err).
						Msg("Could not write last packets after interrupt to Output")
				}
				return
			}
		}
	}
}

func init() {
	Register("bandwidth", new(BandwidthToxic))
}
