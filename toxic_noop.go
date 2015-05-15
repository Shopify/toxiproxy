package main

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct{}

func (t *NoopToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.interrupt:
			return
		case c := <-stub.input:
			if c == nil {
				stub.Close()
				return
			}
			stub.output <- c
		}
	}
}

func init() {
	RegisterToxic("noop", new(NoopToxic))
}
