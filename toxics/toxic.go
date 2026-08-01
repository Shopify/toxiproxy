package toxics

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shopify/toxiproxy/v2/stream"
)

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
	// Defines how packets flow through a ToxicStub.
	// Pipe() blocks until the link is closed or interrupted.
	Pipe(*ToxicStub)
}

type CleanupToxic interface {
	// Cleanup is called before a toxic is removed.
	Cleanup(*ToxicStub)
}

type BufferedToxic interface {
	// Defines the size of buffer this toxic should use
	GetBufferSize() int
}

// ExpectedDelay toxics intentionally add latency to a chunk, so a stub's
// Timeout can account for it instead of mistaking a slow chain for a dead one.
type ExpectedDelay interface {
	ExpectedDelay() time.Duration
}

// DefaultOutputTimeout is the floor added on top of any ExpectedDelay.
const DefaultOutputTimeout = 5 * time.Second

// Stateful toxics store a per-connection state object on the ToxicStub.
// The state is created once when the toxic is added and persists until the
// toxic is removed or the connection is closed.
type StatefulToxic interface {
	// Creates a new object to store toxic state in
	NewState() interface{}
}

type ToxicWrapper struct {
	Toxic      `json:"attributes"`
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	Stream     string           `json:"stream"`
	Toxicity   float32          `json:"toxicity"`
	Direction  stream.Direction `json:"-"`
	Index      int              `json:"-"`
	BufferSize int              `json:"-"`
}

type ToxicStub struct {
	Input     <-chan *stream.StreamChunk
	Output    chan<- *stream.StreamChunk
	State     interface{}
	Interrupt chan struct{}
	running   chan struct{}
	closed    chan struct{}

	// timeoutNanos bounds a blocked Output send before treating the consumer
	// as gone. Accessed via Timeout/SetTimeout: ToxicLink writes it under the
	// collection's lock, Pipe goroutines read it without that lock.
	timeoutNanos atomic.Int64

	// OutputDone, when set, closes exactly when the stub reading Output stops
	// for good. WriteOutput selects on it directly instead of checking it
	// once before sending, so a send already in flight still unblocks the
	// instant the consumer goes away. Nil for a chain's last stub.
	OutputDone <-chan struct{}
}

func NewToxicStub(input <-chan *stream.StreamChunk, output chan<- *stream.StreamChunk) *ToxicStub {
	s := &ToxicStub{
		Interrupt: make(chan struct{}),
		closed:    make(chan struct{}),
		Input:     input,
		Output:    output,
	}
	s.SetTimeout(DefaultOutputTimeout)
	return s
}

// Timeout returns the current send timeout, safe for concurrent use.
func (s *ToxicStub) Timeout() time.Duration {
	return time.Duration(s.timeoutNanos.Load())
}

// SetTimeout updates the send timeout, safe for concurrent use.
func (s *ToxicStub) SetTimeout(d time.Duration) {
	s.timeoutNanos.Store(int64(d))
}

// Begin running a toxic on this stub, can be interrupted.
// Runs a noop toxic randomly depending on toxicity.
func (s *ToxicStub) Run(toxic *ToxicWrapper) {
	s.running = make(chan struct{})
	defer close(s.running)
	randomToxicity := rand.Float32() // #nosec G404 -- was ignored before too
	if randomToxicity < toxic.Toxicity {
		toxic.Pipe(s)
	} else {
		new(NoopToxic).Pipe(s)
	}
}

// WriteOutput allows to write to Output with timeout to avoid deadlocks.
// If duration is 0, then wait until other goroutines finish reading from Output.
func (s *ToxicStub) WriteOutput(p *stream.StreamChunk, d time.Duration) error {
	if d == 0 {
		select {
		case s.Output <- p:
			return nil
		case <-s.OutputDone:
			return fmt.Errorf("output already closed")
		}
	}

	select {
	case s.Output <- p:
		return nil
	case <-s.OutputDone:
		return fmt.Errorf("output already closed")
	case <-time.After(d):
		return fmt.Errorf("timeout: could not write to output in %d seconds", int(d.Seconds()))
	}
}

// Interrupt the flow of data so that the toxic controlling the stub can be replaced.
// Returns true if the stream was successfully interrupted, or false if the stream is closed.
func (s *ToxicStub) InterruptToxic() bool {
	select {
	case <-s.closed:
		return false
	case s.Interrupt <- struct{}{}:
		<-s.running // Wait for the running toxic to exit
		return true
	}
}

func (s *ToxicStub) Closed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

// Done reports when this stub itself stops; see ToxicStub.OutputDone.
func (s *ToxicStub) Done() <-chan struct{} {
	return s.closed
}

func (s *ToxicStub) Close() {
	if !s.Closed() {
		close(s.closed)
		close(s.Output)
	}
}

var (
	ToxicRegistry map[string]Toxic
	registryMutex sync.RWMutex
)

func Register(typeName string, toxic Toxic) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if ToxicRegistry == nil {
		ToxicRegistry = make(map[string]Toxic)
	}
	ToxicRegistry[typeName] = toxic
}

func New(wrapper *ToxicWrapper) Toxic {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	orig, ok := ToxicRegistry[wrapper.Type]
	if !ok {
		return nil
	}
	wrapper.Toxic = reflect.New(reflect.TypeOf(orig).Elem()).Interface().(Toxic)
	if buffered, ok := wrapper.Toxic.(BufferedToxic); ok {
		wrapper.BufferSize = buffered.GetBufferSize()
	} else {
		wrapper.BufferSize = 0
	}
	return wrapper.Toxic
}

func Count() int {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	return len(ToxicRegistry)
}
