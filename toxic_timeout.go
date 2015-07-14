package main

import "time"
import "math/rand"

var noop = new(NoopToxic)

// The TimeoutToxic stops any data from flowing through, and will close the connection after a timeout.
// If the timeout is set to 0, then the connection will not be closed.
type TimeoutToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Timeout int64 `json:"timeout"`
	// If true, use Toxicity
	SometimesToxic bool `json:"sometimesToxic"`
	// 'Toxicity' or probability of timing out 0..1
	Toxicity float32 `json:"toxicity"`
}

func (t *TimeoutToxic) Name() string {
	return "timeout"
}

func (t *TimeoutToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *TimeoutToxic) SetEnabled(enabled bool) {
	t.Enabled = enabled
}

func (t *TimeoutToxic) Pipe(stub *ToxicStub) {
	if t.SometimesToxic && rand.Float32() >= t.Toxicity {
		// just pipe the data through
		noop.Pipe(stub)
	} else {
		// do timeout
		timeout := time.Duration(t.Timeout) * time.Millisecond
		if timeout > 0 {
			select {
			case <-time.After(timeout):
				stub.Close()
				return
			case <-stub.interrupt:
				return
			}
		} else {
			<-stub.interrupt
			return
		}
	}
}
