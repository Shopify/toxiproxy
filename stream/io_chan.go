package stream

import (
	"fmt"
	"io"
	"time"
)

type Direction uint8

const (
	Upstream Direction = iota
	Downstream
	NumDirections
)

// Stores a slice of bytes with its receive timestmap
type StreamChunk struct {
	Data      []byte
	Timestamp time.Time
}

// Implements the io.WriteCloser interface for a chan []byte
type ChanWriter struct {
	output chan<- *StreamChunk
}

func NewChanWriter(output chan<- *StreamChunk) *ChanWriter {
	return &ChanWriter{output}
}

// Write `buf` as a StreamChunk to the channel. The full buffer is always written, and error
// will always be nil. Calling `Write()` after closing the channel will panic.
func (c *ChanWriter) Write(buf []byte) (int, error) {
	packet := &StreamChunk{make([]byte, len(buf)), time.Now()}
	copy(packet.Data, buf) // Make a copy before sending it to the channel
	c.output <- packet
	return len(buf), nil
}

// Close the output channel
func (c *ChanWriter) Close() error {
	close(c.output)
	return nil
}

// Implements the io.Reader interface for a chan []byte
type ChanReader struct {
	input     <-chan *StreamChunk
	interrupt <-chan struct{}
	buffer    []byte
}

var ErrInterrupted = fmt.Errorf("read interrupted by channel")

func NewChanReader(input <-chan *StreamChunk) *ChanReader {
	return &ChanReader{input, make(chan struct{}), []byte{}}
}

// Specify a channel that can interrupt a read if it is blocking.
func (c *ChanReader) SetInterrupt(interrupt <-chan struct{}) {
	c.interrupt = interrupt
}

// Read from the channel into `out`. This will block until data is available,
// and can be interrupted with a channel using `SetInterrupt()`. If the read
// was interrupted, `ErrInterrupted` will be returned.
func (c *ChanReader) Read(out []byte) (int, error) {
	if c.buffer == nil {
		return 0, io.EOF
	}
	n := copy(out, c.buffer)
	c.buffer = c.buffer[n:]
	if len(out) <= len(c.buffer) {
		return n, nil
	} else if n > 0 {
		// We have some data to return, so make the channel read optional
		select {
		case p := <-c.input:
			if p == nil { // Stream was closed
				c.buffer = nil
				if n > 0 {
					return n, nil
				}
				return 0, io.EOF
			}
			n2 := copy(out[n:], p.Data)
			c.buffer = p.Data[n2:]
			return n + n2, nil
		default:
			return n, nil
		}
	}
	var p *StreamChunk
	select {
	case p = <-c.input:
	case <-c.interrupt:
		c.buffer = c.buffer[:0]
		return n, ErrInterrupted
	}
	if p == nil { // Stream was closed
		c.buffer = nil
		return 0, io.EOF
	}
	n2 := copy(out[n:], p.Data)
	c.buffer = p.Data[n2:]
	return n + n2, nil
}
