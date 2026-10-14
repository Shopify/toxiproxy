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
			// Waits as long as the next stage reads, but returns once it is gone
			// (e.g. reset_peer closed itself), so removal can't hang.
			_ = stub.WriteOutput(c, 0)
		}
	}
}

func init() {
	Register("noop", new(NoopToxic))
}
