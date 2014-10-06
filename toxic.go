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
// Toxic's work in a pipeline fashion, and can be chained together.

type Toxic interface {
	Init(*Proxy, io.Reader, io.WriteCloser)
	Pipe()
	Interrupt()
	Pause()
	Resume()
}

// BaseToxic is to allow for common fields between all toxics.
type BaseToxic struct {
	Enabled   bool `json:"enabled"`
	proxy     *Proxy
	input     io.Reader
	output    io.WriteCloser
	interrupt chan struct{}
	pauseOut  chan bool // true: stop output, false: resume output
}

func (t *BaseToxic) Init(proxy *Proxy, input io.Reader, output io.WriteCloser) {
	t.proxy = proxy
	t.interrupt = make(chan struct{})
	t.input = input
	t.output = output
}

// Interrupt the flow of data through the toxic so that the toxic
// can be replaced or removed.
func (t *BaseToxic) Interrupt() {
	t.interrupt <- struct{}{}
}

// Pause any calls to the toxic's output writer so it can be changed.
func (t *BaseToxic) Pause() {
	t.pauseOut <- true
}

func (t *BaseToxic) Resume() {
	t.pauseOut <- false
}

// StreamBuffer is used for chaining Toxic's together. It implements both the
// io.Reader and io.WriteCloser interfaces, so it can be passed into Toxics.
// Read() will block until data is written to the buffer rather than
// returning EOF if Read() is called faster than Write().
//
// The Toxic pipeline looks like this: (|| == StreamBuffer)
//
// Input > Toxic || Toxic || Toxic > Output
//
type StreamBuffer struct {
	buffer      *bytes.Buffer
	writeSignal chan bool // true: More data available, false: EOF
}

func (s *StreamBuffer) Read(buf []byte) (int, error) {
	<-s.writeSignal // Wait until either EOF or some data is ready
	return s.buffer.Read(buf)
}

func (s *StreamBuffer) Write(buf []byte) (int, error) {
	n, err := s.buffer.Write(buf)
	if n > 0 {
		s.writeSignal <- true
	}
	return n, err
}

func (s *StreamBuffer) Close() error {
	s.writeSignal <- false
	return nil
}

func NewStreamBuffer() *StreamBuffer {
	return &StreamBuffer{
		buffer:      bytes.NewBuffer([]byte{}),
		writeSignal: make(chan bool, 10),
	}
}

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct {
	BaseToxic
}

func (t *NoopToxic) Pipe() {
	bytes, err := copy(&t.BaseToxic, nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"toxic":    "NoopToxic",
			"name":     t.proxy.Name,
			"upstream": t.proxy.Upstream,
			"bytes":    bytes,
			"err":      err,
		}).Warn("Client or source terminated")
	}
	t.output.Close()
}

// The LatencyToxic passes data through with the specified latency and jitter added.
type LatencyToxic struct {
	BaseToxic
	Latency time.Duration `json:"latency"`
	Jitter  time.Duration `json:"jitter"`
}

func (t *LatencyToxic) Pipe() {
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
	bytes, err := copy(&t.BaseToxic, latency)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"toxic":    "LatencyToxic",
			"name":     t.proxy.Name,
			"upstream": t.proxy.Upstream,
			"bytes":    bytes,
			"err":      err,
		}).Warn("Client or source terminated")
	}
	t.output.Close()
	running = false
	select {
	case <-latency: // Optionally read from latency to unblock the go routine
	default:
	}
}

// Copy breaks up the input stream into random packets of size 1-32k bytes. Each
// packet is then delayed for a time specified by the latency channel.
// At any time the stream can be interrupted, and the function will return.
// The stream can be paused without interrupting the latency state as well.
// This copy function is a modified version of io.Copy()
func copy(toxic *BaseToxic, latency <-chan time.Duration) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		if latency != nil {
			// Delay the packet for a duration specified by the latency channel.
			sleep := <-latency
			wait := time.After(sleep)
			select {
			case <-wait:
			case <-toxic.interrupt:
				break
			}
		} else {
			select {
			case <-toxic.interrupt:
				break
			default:
			}
		}
		nr, er := toxic.input.Read(buf[0:rand.Intn(32*1024)]) // Read a random packet size
		if nr > 0 {
			select {
			case pause := <-toxic.pauseOut:
				for pause {
					pause = <-toxic.pauseOut
				}
			default:
			}
			nw, ew := toxic.output.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
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
	return written, err
}
