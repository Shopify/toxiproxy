package toxics

import (
	"math/rand"
	"reflect"
	"sync"

	"github.com/Shopify/toxiproxy/stream"
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
	// Defines how packets flow through a ToxicStub. Pipe() blocks until the link is closed or interrupted.
	Pipe(*ToxicStub)
}

type BufferedToxic interface {
	// Defines the size of buffer this toxic should use
	GetBufferSize() int
}

type ToxicWrapper struct {
	Toxic
	Name       string
	Type       string
	Stream     string
	Toxicity   float32
	Direction  stream.Direction
	Index      int
	BufferSize int
}

// Returns a flattened map of the toxic for use with json
func (w *ToxicWrapper) GetMap() map[string]interface{} {
	result := make(map[string]interface{})
	ref := reflect.ValueOf(w.Toxic).Elem()
	for i := 0; i < ref.NumField(); i++ {
		result[ref.Type().Field(i).Tag.Get("json")] = ref.Field(i).Interface()
	}
	result["name"] = w.Name
	result["type"] = w.Type
	result["stream"] = w.Stream
	result["toxicity"] = w.Toxicity
	return result
}

type ToxicStub struct {
	Input     <-chan *stream.StreamChunk
	Output    chan<- *stream.StreamChunk
	Interrupt chan struct{}
	running   chan struct{}
	closed    chan struct{}
}

func NewToxicStub(input <-chan *stream.StreamChunk, output chan<- *stream.StreamChunk) *ToxicStub {
	return &ToxicStub{
		Interrupt: make(chan struct{}),
		closed:    make(chan struct{}),
		Input:     input,
		Output:    output,
	}
}

// Begin running a toxic on this stub, can be interrupted.
// Runs a noop toxic randomly depending on toxicity
func (s *ToxicStub) Run(toxic *ToxicWrapper) {
	s.running = make(chan struct{})
	defer close(s.running)
	if rand.Float32() < toxic.Toxicity {
		toxic.Pipe(s)
	} else {
		new(NoopToxic).Pipe(s)
	}
}

// Interrupt the flow of data so that the toxic controlling the stub can be replaced.
// Returns true if the stream was successfully interrupted.
func (s *ToxicStub) InterruptToxic() bool {
	select {
	case <-s.closed:
		return false
	case s.Interrupt <- struct{}{}:
		<-s.running // Wait for the running toxic to exit
		return true
	}
}

func (s *ToxicStub) Close() {
	close(s.closed)
	close(s.Output)
}

var ToxicRegistry map[string]Toxic
var registryMutex sync.RWMutex

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
