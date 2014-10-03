package main

import (
	"bytes"
	"io"
	"math/rand"
	"time"

	"github.com/Sirupsen/logrus"
)

// A Toxic is something that can be attatched to a link to modify the way
// data can be passed through (for example, by adding latency)
//
//       Toxic
//         v
// Client <-> Upstream
//
// Toxic's work in a pipeline fashion, and can be chained together if necessary.

type Toxic interface {
	Pipe(*pipe)
}

// Pipe is used for chaining Toxic's together. It implements the io.Reader
// interface, so it can be passed in to another pipe.
type pipe struct {
	proxy     *Proxy
	input     io.Reader
	output    *bytes.Buffer
	outSignal chan bool
	interrupt chan bool
}

func (p *pipe) Read(buf []byte) (int, error) {
	<-p.outSignal // Wait until either EOF or some data is ready
	return p.output.Read(buf)
}

// Start piping data through using the specified toxic
func (p *pipe) Start(toxic Toxic) {
	go toxic.Pipe(p)
}

// Interrupt the flow of data through the pipe so that the toxic
// can be replaced or a new pipe can be added.
func (p *pipe) Interrupt() {
	p.interrupt <- true
}

func NewPipe(proxy *Proxy, input io.Reader) *pipe {
	return &pipe{
		proxy:     proxy,
		input:     input,
		output:    bytes.NewBuffer([]byte{}),
		outSignal: make(chan bool),
		interrupt: make(chan bool),
	}
}

// BaseToxic is to allow for common fields between all toxics.
type BaseToxic struct {
	Enabled bool `json:"enabled"`
}

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct {
	BaseToxic
}

func (t *NoopToxic) Pipe(p *pipe) {
	bytes, err := copy(p.input, p.output, nil, p.interrupt, p.outSignal)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"toxic":    "NoopToxic",
			"name":     p.proxy.Name,
			"upstream": p.proxy.Upstream,
			"bytes":    bytes,
			"err":      err,
		}).Warn("Client or source terminated")
	}
}

// The LatencyToxic passes data through with the specified latency and jitter added.
type LatencyToxic struct {
	BaseToxic
	Latency time.Duration `json:"latency"`
	Jitter  time.Duration `json:"jitter"`
}

func (t *LatencyToxic) Pipe(p *pipe) {
	running := true
	latency := make(chan time.Duration)
	go func() {
		for running {
			// Delay = t.Latency +/- t.Jitter
			delay := t.Latency
			jitter := int64(t.Jitter)
			if jitter > 0 {
				delay += time.Duration(rand.Int63n(jitter*2) - jitter)
			}
			latency <- delay
		}
	}()
	bytes, err := copy(p.input, p.output, latency, p.interrupt, p.outSignal)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"toxic":    "LatencyToxic",
			"name":     p.proxy.Name,
			"upstream": p.proxy.Upstream,
			"bytes":    bytes,
			"err":      err,
		}).Warn("Client or source terminated")
	}
	running = false
	select {
	case <-latency: // Optionally read from latency to unblock the go routine
	default:
	}
}

// Copy breaks up the input stream into random packets of size 1-32k bytes. Each
// packet is then delayed for a time specified by the latency channel.
// At any time the stream can be interrupted, and the function will return.
// This copy function is a modified version of io.Copy()
func copy(src io.Reader, dst io.Writer, latency <-chan time.Duration, interrupt <-chan bool, outSignal chan<- bool) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		if latency != nil {
			// Delay the packet for a duration specified by the latency channel.
			sleep := <-latency
			wait := time.After(sleep)
			select {
			case <-wait:
			case <-interrupt:
				break
			}
		} else {
			select {
			case <-interrupt:
				break
			default:
			}
		}
		nr, er := src.Read(buf[0:rand.Intn(32*1024)]) // Read a random packet size
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				outSignal <- true
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	outSignal <- false
	return written, err
}
