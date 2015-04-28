package main

import (
	"io"
	"time"
)

// Simulates a TCP packet by storing a slice of bytes with the receive timestmap
type Packet struct {
	data      []byte
	timestamp time.Time
}

// Implements the io.WriteCloser interface for a chan []byte
type ChanWriter struct {
	output chan<- *Packet
}

func NewChanWriter(output chan<- *Packet) *ChanWriter {
	return &ChanWriter{output}
}

func (c *ChanWriter) Write(buf []byte) (int, error) {
	packet := &Packet{make([]byte, len(buf)), time.Now()}
	copy(packet.data, buf) // Make a copy before sending it to the channel
	c.output <- packet
	return len(buf), nil
}

func (c *ChanWriter) Close() error {
	close(c.output)
	return nil
}

// Implements the io.Reader interface for a chan []byte
type ChanReader struct {
	input  <-chan *Packet
	buffer []byte
}

func NewChanReader(input <-chan *Packet) *ChanReader {
	return &ChanReader{input, []byte{}}
}

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
				return n, io.EOF
			}
			n2 := copy(out[n:], p.data)
			c.buffer = p.data[n2:]
			return n + n2, nil
		default:
			return n, nil
		}
	}
	p := <-c.input
	if p == nil { // Stream was closed
		c.buffer = nil
		return 0, io.EOF
	}
	n2 := copy(out[n:], p.data)
	c.buffer = p.data[n2:]
	return n + n2, nil
}
