package main

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct{}

func (t *NoopToxic) Name() string {
	return "noop"
}

func (t *NoopToxic) IsEnabled() bool {
	return true
}

func (t *NoopToxic) SetEnabled(enabled bool) {}

func (t *NoopToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.interrupt:
			return
		case p := <-stub.input:
			if p == nil {
				stub.Close()
				return
			}
			stub.output <- p
		}
	}
}
