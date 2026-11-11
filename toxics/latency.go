package toxics

import (
	"math/rand"
	"time"
)

// The LatencyToxic passes data through with the a delay of latency +/- jitter added.
type LatencyToxic struct {
	// Times in milliseconds
	Latency int64 `json:"latency"`
	Jitter  int64 `json:"jitter"`
}

func (t *LatencyToxic) GetBufferSize() int {
	return 1024
}

func (t *LatencyToxic) delay() time.Duration {
	// Delay = t.Latency +/- t.Jitter
	delay := t.Latency
	jitter := t.Jitter
	if jitter > 0 {
		// #nosec G404 -- was ignored before too
		delay += rand.Int63n(jitter*2) - jitter
	}
	// A jitter larger than the latency can push the delay below zero, which
	// means no delay rather than sending the data early.
	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay) * time.Millisecond
}

func (t *LatencyToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}
			delay := t.delay()
			sleep := delay - time.Since(c.Timestamp)
			select {
			case <-time.After(sleep):
				// Advance by the full delay rather than the remaining sleep, so a
				// following toxic measures from when this chunk was due to be sent.
				// Otherwise time the chunk spent queued behind earlier chunks would
				// be deducted from the next toxic's delay.
				c.Timestamp = c.Timestamp.Add(delay)
				_ = stub.WriteOutput(c, 0)
			case <-stub.Interrupt:
				// Exit fast without applying latency.
				_ = stub.WriteOutput(c, 0) // Don't drop any data on the floor
				return
			}
		}
	}
}

func init() {
	Register("latency", new(LatencyToxic))
}
