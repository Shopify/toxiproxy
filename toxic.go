package main

import (
	"math/rand"
	"time"
)

// A Toxic is something that can be attatched to a link to modify the way
// data can be passed through (for example, by adding latency)
//
//              Toxic
//                v
// Client <-> ToxicStub <-> Upstream
//
// Toxic's work in a pipeline fashion, and can be chained together
// with StreamBuffers. The toxic itself only defines the settings and
// Pipe() function definition, and uses the ToxicStub struct to store
// per-connection information. This allows the same toxic to be used
// for multiple connections.

type Toxic interface {
	IsEnabled() bool
	Pipe(*ToxicStub)
}

type ToxicStub struct {
	proxy     *Proxy
	input     <-chan []byte
	output    chan<- []byte
	interrupt chan struct{}
}

func NewToxicStub(proxy *Proxy, input <-chan []byte, output chan<- []byte) *ToxicStub {
	return &ToxicStub{
		proxy:     proxy,
		interrupt: make(chan struct{}),
		input:     input,
		output:    output,
	}
}

// Interrupt the flow of data through the toxic so that the toxic
// can be replaced or removed.
func (s *ToxicStub) Interrupt() {
	s.interrupt <- struct{}{}
}

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct{}

func (t *NoopToxic) IsEnabled() bool {
	return true
}

func (t *NoopToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.interrupt:
			return
		case buf := <-stub.input:
			if buf == nil {
				close(stub.output)
				return
			}
			stub.output <- buf
		}
	}
}

// The LatencyToxic passes data through with the specified latency and jitter added.
type LatencyToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Latency int64 `json:"latency"`
	Jitter  int64 `json:"jitter"`
}

func (t *LatencyToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *LatencyToxic) getDelay() time.Duration {
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
		case buf := <-stub.input:
			if buf == nil {
				close(stub.output)
				return
			}
			sleep := t.getDelay()
			select {
			case <-time.After(sleep):
				stub.output <- buf
			case <-stub.interrupt:
				stub.output <- buf // Don't drop any data on the floor
				return
			}
		}
	}
}
