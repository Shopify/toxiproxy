package main

import "time"

// The SlowCloseToxic stops the TCP connection from closing until after a delay.
type SlowCloseToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Delay int64 `json:"delay"`
}

func (t *SlowCloseToxic) Name() string {
	return "slow_close"
}

func (t *SlowCloseToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *SlowCloseToxic) Pipe(stub *ToxicStub) bool {
	for {
		select {
		case <-stub.interrupt:
			return true
		case buf := <-stub.input:
			if buf == nil {
				delay := time.Duration(t.Delay) * time.Millisecond
				select {
				case <-time.After(delay):
					close(stub.output)
					return false
				case <-stub.interrupt:
					return true
				}
			}
			stub.output <- buf
		}
	}
}
