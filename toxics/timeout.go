package toxics

import "time"

// The TimeoutToxic stops any data from flowing through, and will close the connection after a timeout.
// If the timeout is set to 0, then the connection will not be closed.
type TimeoutToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Timeout int64 `json:"timeout"`
}

func (t *TimeoutToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *TimeoutToxic) Pipe(stub *ToxicStub) bool {
	timeout := time.Duration(t.Timeout) * time.Millisecond
	if timeout > 0 {
		select {
		case <-time.After(timeout):
			close(stub.output)
			return false
		case <-stub.interrupt:
			return true
		}
	} else {
		<-stub.interrupt
		return true
	}
}
