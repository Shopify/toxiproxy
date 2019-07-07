package toxics

import "time"

// The Timeout stops any data from flowing through, and will close the connection after a timeout.
// If the timeout is set to 0, then the connection will not be closed.
type Timeout struct {
	// Times in milliseconds
	Timeout int64 `json:"timeout"`
}

func (t *Timeout) Pipe(stub *Stub) {
	timeout := time.Duration(t.Timeout) * time.Millisecond
	if timeout > 0 {
		for {
			select {
			case <-time.After(timeout):
				stub.Close()
				return
			case <-stub.Interrupt:
				return
			case c := <-stub.Input:
				if c == nil {
					stub.Close()
					return
				}
				// Drop the data on the ground.
			}
		}
	} else {
		for {
			select {
			case <-stub.Interrupt:
				return
			case c := <-stub.Input:
				if c == nil {
					stub.Close()
					return
				}
				// Drop the data on the ground.
			}
		}
	}
}

func (t *Timeout) Cleanup(stub *Stub) {
	stub.Close()
}

func init() {
	Register("timeout", new(Timeout))
}
