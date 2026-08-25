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
			// Bounded: a plain `stub.Output <- c` blocks forever once the next
			// toxic stops reading on its own (e.g. reset_peer after one
			// chunk), which freezes removal too.
			_ = stub.WriteOutput(c, stub.Timeout())
		}
	}
}

func init() {
	Register("noop", new(NoopToxic))
}
