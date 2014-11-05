package toxics

// A Toxic is something that can be attatched to a link to modify the way
// data can be passed through (for example, by adding latency)
//
//              Toxic
//                v
// Client <-> ToxicStub <-> Upstream
//
// Toxic's work in a pipeline fashion, and can be chained together
// with channels. The toxic itself only defines the settings and
// Pipe() function definition, and uses the ToxicStub struct to store
// per-connection information. This allows the same toxic to be used
// for multiple connections.

type Toxic interface {
	IsEnabled() bool

	// Returns true if interrupted, false if closed
	Pipe(*ToxicStub) bool
}

// Constants used to define which order toxics are chained in.
const (
	TimeoutIndex = iota
	LatencyIndex
	SlowCloseIndex
	MaxToxics
)

type ToxicStub struct {
	input     <-chan []byte
	output    chan<- []byte
	interrupt chan struct{}
}

func NewToxicStub(input <-chan []byte, output chan<- []byte) *ToxicStub {
	return &ToxicStub{
		interrupt: make(chan struct{}),
		input:     input,
		output:    output,
	}
}

// Interrupt the flow of data through the toxic so that the toxic
// can be replaced or removed.
func (s *ToxicStub) Interrupt() {
	s.interrupt <- struct{}{}
}

// Unblocks writes to the interrupt channel until the specified channel is written to.
func (s *ToxicStub) Unblock(until <-chan struct{}) {
	for {
		select {
		case <-s.interrupt:
		case <-until:
			return
		}
	}
}
