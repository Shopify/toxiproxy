package toxics

// The Noop passes all data through without any toxic effects.
type Noop struct{}

func (t *Noop) Pipe(stub *Stub) {
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
	Register("noop", new(Noop))
}
