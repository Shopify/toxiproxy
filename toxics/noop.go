package toxics

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct{}

func (t *NoopToxic) Pipe(stub *ToxicStub) {
	for {
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

func init() {
	Register("noop", new(NoopToxic))
}
