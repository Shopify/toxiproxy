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

func (t *LatencyToxic) SetEnabled(enabled bool) {
	t.Enabled = enabled
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
		case p := <-stub.input:
			if p == nil {
				stub.Close()
				return
			}
			sleep := t.delay() - time.Now().Sub(p.timestamp)
			select {
			case <-time.After(sleep):
				stub.output <- p
			case <-stub.interrupt:
				stub.output <- p // Don't drop any data on the floor
				return
			}
		}
	}
}
