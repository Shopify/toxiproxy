package main

import "io"

// Implements the io.WriteCloser interface for a chan []byte
type ChanWriter struct {
	output chan<- []byte
}

func NewChanWriter(output chan<- []byte) *ChanWriter {
	return &ChanWriter{output}
}

func (c *ChanWriter) Write(buf []byte) (int, error) {
	buf2 := make([]byte, len(buf))
	copy(buf2, buf) // Make a copy before sending it to the channel
	c.output <- buf2
	return len(buf), nil
}

func (c *ChanWriter) Close() error {
	close(c.output)
	return nil
}

// Implements the io.Reader interface for a chan []byte
type ChanReader struct {
	input  <-chan []byte
	buffer []byte
}

func NewChanReader(input <-chan []byte) *ChanReader {
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
		case c.buffer = <-c.input:
			if c.buffer == nil { // Stream was closed
				return n, io.EOF
			}
			n2 := copy(out[n:], c.buffer)
			c.buffer = c.buffer[n2:]
			return n + n2, nil
		default:
			return n, nil
		}
	}
	c.buffer = <-c.input
	if c.buffer == nil { // Stream was closed
		return 0, io.EOF
	}
	n2 := copy(out[n:], c.buffer)
	c.buffer = c.buffer[n2:]
	return n + n2, nil
}
