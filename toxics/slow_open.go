package toxics

import (
	"time"
)

// The SlowOpenToxic adds a delay to the first data packet of a new TCP
// connection, to simulate the delay experienced by a calling application
// due to the TCP handshake.
//
// For context: the TCP handshake is not covered by LatencyToxic
// and cannot be, since (in the current Toxiproxy architecture) it is
// handled by the OS network stack.
// This means that you cannot accurately simulate a latency occurring
// during the connect phase and thus test behaviors related to connection
// timeouts.
// However, if your goal is to simulate the delays experienced by the
// caller at the application level, using this toxic in addition to
// LatencyToxic will model them more accurately than using LatencyToxic
// alone.
type SlowOpenToxic struct {
	// Times in milliseconds
	Delay int64 `json:"delay"`
}

type SlowOpenToxicState struct {
	Warm bool
}

func (t *SlowOpenToxic) GetBufferSize() int {
	return 1024
}

func (t *SlowOpenToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*SlowOpenToxicState)

	for {
		if !state.Warm {
			select {
			case <-stub.Interrupt:
				return
			case c := <-stub.Input:
				if c == nil {
					stub.Close()
					return
				}

				delay := time.Duration(t.Delay) * time.Millisecond
				state.Warm = true

				select {
				case <-time.After(delay):
					c.Timestamp = c.Timestamp.Add(delay)
					stub.Output <- c
				case <-stub.Interrupt:
					stub.Output <- c
					return
				}
			}
		} else {
			select {
			case <-stub.Interrupt:
				return
			case c := <-stub.Input:
				if c == nil {
					stub.Close()
					return
				}
				stub.Output <- c
			}
		}
	}
}

func (t *SlowOpenToxic) NewState() interface{} {
	return new(SlowOpenToxicState)
}

func init() {
	Register("slow_open", new(SlowOpenToxic))
}
