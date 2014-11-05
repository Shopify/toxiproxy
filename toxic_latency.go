package main

import (
	"math/rand"
	"time"
)

// The LatencyToxic passes data through with the a delay of latency +/- jitter added.
type LatencyToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Latency int64 `json:"latency"`
	Jitter  int64 `json:"jitter"`
}

func (t *LatencyToxic) Name() string {
	return "latency"
}

func (t *LatencyToxic) IsEnabled() bool {
	return t.Enabled
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

func (t *LatencyToxic) Pipe(stub *ToxicStub) bool {
	for {
		select {
		case <-stub.interrupt:
			return true
		case buf := <-stub.input:
			if buf == nil {
				close(stub.output)
				return false
			}
			sleep := t.delay()
			select {
			case <-time.After(sleep):
				stub.output <- buf
			case <-stub.interrupt:
				stub.output <- buf // Don't drop any data on the floor
				return true
			}
		}
	}
}
