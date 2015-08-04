package toxics

import "time"

// The SlowCloseToxic stops the TCP connection from closing until after a delay.
type SlowCloseToxic struct {
	// Times in milliseconds
	Delay int64 `json:"delay"`
}

func (t *SlowCloseToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				delay := time.Duration(t.Delay) * time.Millisecond
				select {
				case <-time.After(delay):
					stub.Close()
					return
				case <-stub.Interrupt:
					return
				}
			}
			stub.Output <- c
		}
	}
}

func init() {
	Register("slow_close", new(SlowCloseToxic))
}
