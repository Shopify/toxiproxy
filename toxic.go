package main

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
	// Return the unique name of the toxic, as used by the json api.
	Name() string

	// Returns true if the toxic is enabled. Disabled toxics are not used and are replaced with NoopToxics.
	IsEnabled() bool

	// Defines how packets flow through a ToxicStub. Pipe() blocks until the link is closed or interrupted.
	Pipe(*ToxicStub)
}

type ToxicStub struct {
	input     <-chan []byte
	output    chan<- []byte
	interrupt chan struct{}
	closed    chan struct{}
}

func NewToxicStub(input <-chan []byte, output chan<- []byte) *ToxicStub {
	return &ToxicStub{
		interrupt: make(chan struct{}),
		closed:    make(chan struct{}),
		input:     input,
		output:    output,
	}
}

// Interrupt the flow of data so that the toxic controlling the stub can be replaced.
// Returns true if the stream was successfully interrupted.
func (s *ToxicStub) Interrupt() bool {
	select {
	case <-s.closed:
		return false
	case s.interrupt <- struct{}{}:
		return true
	}
}

func (s *ToxicStub) Close() {
	close(s.output)
	close(s.closed)
}
