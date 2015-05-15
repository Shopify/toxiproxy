package main

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
		case <-stub.interrupt:
			return
		case c := <-stub.input:
			if c == nil {
				stub.Close()
				return
			}
			sleep := t.delay() - time.Since(c.timestamp)
			select {
			case <-time.After(sleep):
				stub.output <- c
			case <-stub.interrupt:
				stub.output <- c // Don't drop any data on the floor
				return
			}
		}
	}
}

func init() {
	RegisterToxic("latency", new(LatencyToxic))
}
