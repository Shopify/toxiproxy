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
	jitter := int64(t.Jitter)
	if jitter > 0 {
		delay += rand.Int63n(jitter*2) - jitter
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
			sleep := t.delay() - time.Since(c.Timestamp)
			select {
			case <-time.After(sleep):
				c.Timestamp = c.Timestamp.Add(sleep)
				stub.Output <- c
			case <-stub.Interrupt:
				// Exit fast without applying latency.
				stub.Output <- c // Don't drop any data on the floor
				return
			}
		}
	}
}

func init() {
	Register("latency", new(LatencyToxic))
}
