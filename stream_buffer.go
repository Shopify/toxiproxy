package main

import "io"

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
	input  chan []byte
	buffer []byte
}

func NewStreamBuffer() *StreamBuffer {
	return &StreamBuffer{
		input:  make(chan []byte),
		buffer: []byte{},
	}
}

func (s *StreamBuffer) Read(out []byte) (int, error) {
	if s.buffer == nil {
		return 0, io.EOF
	}
	if len(out) <= len(s.buffer) {
		n := copy(out, s.buffer)
		s.buffer = s.buffer[n:]
		return n, nil
	}
	n := copy(out, s.buffer)
	s.buffer = s.buffer[n:]
	if n > 0 {
		// We have some data to return, so make the channel read optional
		select {
		case s.buffer = <-s.input:
			if s.buffer == nil { // Stream was closed
				return n, io.EOF
			}
			n2 := copy(out[n:], s.buffer)
			s.buffer = s.buffer[n2:]
			return n + n2, nil
		default:
			return n, nil
		}
	}
	s.buffer = <-s.input
	if s.buffer == nil { // Stream was closed
		return 0, io.EOF
	}
	n2 := copy(out[n:], s.buffer)
	s.buffer = s.buffer[n2:]
	return n + n2, nil
}

func (s *StreamBuffer) Write(buf []byte) (int, error) {
	buf2 := make([]byte, len(buf))
	// Some stream sources such as HTTP clients reuse buffers. This means
	// that the buffer may change before it is read out again, causing
	// either missing or duplicate data on the output.
	copy(buf2, buf)
	s.input <- buf2
	return len(buf), nil
}

func (s *StreamBuffer) Close() error {
	close(s.input)
	return nil
}
