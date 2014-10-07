package main

import (
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
}

// BaseToxic is to allow for common fields between all toxics.
type BaseToxic struct {
	Enabled   bool `json:"enabled"`
	proxy     *Proxy
	input     io.Reader
	output    io.WriteCloser
	interrupt chan struct{}
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

// The NoopToxic passes all data through without any toxic effects.
type NoopToxic struct {
	BaseToxic
}

func (t *NoopToxic) Pipe() {
	bytes, err := toxicCopy(&t.BaseToxic, nil)
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
	bytes, err := toxicCopy(&t.BaseToxic, latency)
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

// toxicCopy() breaks up the input stream into random packets of size 1-32k bytes. Each
// packet is then delayed for a time specified by the latency channel.
// At any time the stream can be interrupted, and the function will return.
// This copy function is a modified version of io.Copy()
func toxicCopy(toxic *BaseToxic, latency <-chan time.Duration) (written int64, err error) {
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
		nr, er := toxic.input.Read(buf[0:1024]) // Read a random packet size
		if nr > 0 {
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
