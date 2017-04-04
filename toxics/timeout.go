package toxics

import "time"

// The TimeoutToxic stops any data from flowing through, and will close the connection after a timeout.
// If the timeout is set to 0, then the connection will not be closed.
type TimeoutToxic struct {
	// Times in milliseconds
	Timeout int64 `json:"timeout"`
}

func (t *TimeoutToxic) Pipe(stub *ToxicStub) {
	timeout := time.Duration(t.Timeout) * time.Millisecond
	if timeout > 0 {
		for {
			select {
			case <-time.After(timeout):
				stub.Close()
				return
			case <-stub.Interrupt:
				return
			case <-stub.Input:
				// Drop the data on the ground.
			}
		}
	} else {
		for {
			select {
			case <-stub.Interrupt:
				return
			case <-stub.Input:
				// Drop the data on the ground.
			}
		}
	}
}

func (t *TimeoutToxic) Cleanup(stub *ToxicStub) {
	stub.Close()
}

func init() {
	Register("timeout", new(TimeoutToxic))
}
