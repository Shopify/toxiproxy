package main

import "time"

// The SlowCloseToxic stops the TCP connection from closing until after a delay.
type SlowCloseToxic struct {
	// Times in milliseconds
	Delay int64 `json:"delay"`
}

func (t *SlowCloseToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.interrupt:
			return
		case c := <-stub.input:
			if c == nil {
				delay := time.Duration(t.Delay) * time.Millisecond
				select {
				case <-time.After(delay):
					stub.Close()
					return
				case <-stub.interrupt:
					return
				}
			}
			stub.output <- c
		}
	}
}

func init() {
	RegisterToxic("slow_close", new(SlowCloseToxic))
}
