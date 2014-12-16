package main

import "time"

// The TimeoutToxic stops any data from flowing through, and will close the connection after a timeout.
// If the timeout is set to 0, then the connection will not be closed.
type TimeoutToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Timeout int64 `json:"timeout"`
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
