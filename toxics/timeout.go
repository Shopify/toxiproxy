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
		select {
		case <-time.After(timeout):
			stub.Close()
			return
		case <-stub.Interrupt:
			return
		}
	} else {
		<-stub.Interrupt
		return
	}
}

func init() {
	Register("timeout", new(TimeoutToxic))
}
