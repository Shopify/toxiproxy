package main

import "time"

// The BandwidthToxic passes data through at a limited rate
type BandwidthToxic struct {
	Enabled bool `json:"enabled"`
	// Rate in KB/s
	Rate int64 `json:"rate"`
}

func (t *BandwidthToxic) Name() string {
	return "bandwidth"
}

func (t *BandwidthToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *BandwidthToxic) SetEnabled(enabled bool) {
	t.Enabled = enabled
}

func (t *BandwidthToxic) Pipe(stub *ToxicStub) {
	var sleep time.Duration = 0
	for {
		select {
		case <-stub.interrupt:
			return
		case p := <-stub.input:
			if p == nil {
				stub.Close()
				return
			}
			if t.Rate <= 0 {
				sleep = 0
			} else {
				sleep += time.Duration(len(p.data)) * time.Millisecond / time.Duration(t.Rate)
			}
			// If the rate is low enough, split the packet up and send in 100 millisecond intervals
			for int64(len(p.data)) > t.Rate*100 {
				select {
				case <-time.After(100 * time.Millisecond):
					stub.output <- &StreamChunk{p.data[:t.Rate*100], p.timestamp}
					p.data = p.data[t.Rate*100:]
					sleep -= 100 * time.Millisecond
				case <-stub.interrupt:
					stub.output <- p // Don't drop any data on the floor
					return
				}
			}
			start := time.Now()
			select {
			case <-time.After(sleep):
				// time.After only seems to have ~1ms prevision, so offset the next sleep by the error
				sleep -= time.Now().Sub(start)
				stub.output <- p
			case <-stub.interrupt:
				stub.output <- p // Don't drop any data on the floor
				return
			}
		}
	}
}
