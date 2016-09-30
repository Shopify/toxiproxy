package stream

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/Sirupsen/logrus"
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
	if len(buf) == 0 {
		return 0, nil
	}

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
	if len(out) == n {
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

// ToxicStub can't be imported due to an import loop, so we use an interface instead
type Closer interface {
	Close()
}

// Reader:
// chan []byte ->  ChanReader -> TeeReader  ->    Read()   -> output
//                                   V              ^
//                             bytes.Buffer -> bytes.Reader
//
// Writer:
// chan []byte <- ChanWriter <- Write() <- input
type ChanReadWriter struct {
	buffer    *bytes.Buffer
	bufReader *bytes.Reader
	reader    *ChanReader
	writer    *ChanWriter
	tee       io.Reader
	closer    Closer
}

// Handles errors returned by Read(). Returns true if the channel has closed and the caller should exit.
// Unknown errors will flush all data since the last checkpoint to the writer and return false so the
// caller can handle the error.
func (c *ChanReadWriter) HandleError(err error) bool {
	if err == ErrInterrupted {
		c.Rollback()
		return true
	} else if err == io.EOF || err == io.ErrUnexpectedEOF {
		c.Rollback()
		c.Flush()
		if c.closer != nil {
			c.closer.Close()
		}
		return true
	} else if err != nil {
		c.Rollback()
		c.Flush()
		logrus.Warn("Read error in toxic: ", err)
	}
	return false
}

// Reads from the input channel either directly, or from a buffer if Rollback() has been called.
func (c *ChanReadWriter) Read(out []byte) (int, error) {
	if c.bufReader != nil {
		n, err := c.bufReader.Read(out)
		if err == io.EOF {
			c.bufReader = nil
			if n > 0 {
				return n, nil
			} else {
				return c.tee.Read(out)
			}
		}
		return n, err
	} else {
		return c.tee.Read(out)
	}
}

// Writes directly to the output channel and sets a checkpoint in the reader.
func (c *ChanReadWriter) Write(buf []byte) (int, error) {
	n, err := c.writer.Write(buf)
	return n, err
}

// Flushes all buffers in the reader and writes them to the output channel.
func (c *ChanReadWriter) Flush() {
	n := 0
	if c.bufReader != nil {
		n = c.bufReader.Len()
	}
	buf := make([]byte, n+len(c.reader.buffer))
	if n > 0 {
		c.bufReader.Read(buf[:n])
	}
	if len(buf[n:]) > 0 {
		c.reader.Read(buf[n:])
	}
	c.writer.Write(buf)
	c.bufReader = nil
	c.buffer.Reset()
}

// Sets a checkpoint in the reader. A call to Rollback() will begin reading from this point.
// If offset is negative, the checkpoint will be set N bytes before the current position.
// If the offset is positive, the checkpoint will be set N bytes after the previous checkpoint.
// An offset of 0 will set the checkpoint to the current position.
func (c *ChanReadWriter) Checkpoint(offset int) {
	current := c.buffer.Len()
	if c.bufReader != nil {
		current = int(c.bufReader.Size()) - c.bufReader.Len()
	}

	n := current
	if offset > 0 {
		n = offset
	} else {
		n = current + offset
	}

	if n >= current {
		c.buffer.Reset()
	} else {
		c.buffer.Next(n)
	}
}

// Rolls back the reader to start from the last checkpoint.
func (c *ChanReadWriter) Rollback() {
	c.bufReader = bytes.NewReader(c.buffer.Bytes())
}

func (c *ChanReadWriter) SetOutput(output chan<- *StreamChunk) {
	c.writer.output = output
}

func (c *ChanReadWriter) SetInterrupt(interrupt <-chan struct{}) {
	c.reader.SetInterrupt(interrupt)
}

func NewChanReadWriter(input <-chan *StreamChunk, output chan<- *StreamChunk, stub Closer) *ChanReadWriter {
	rw := &ChanReadWriter{
		buffer: bytes.NewBuffer(make([]byte, 0, 32*1024)),
		reader: NewChanReader(input),
		writer: NewChanWriter(output),
		closer: stub,
	}
	rw.tee = io.TeeReader(rw.reader, rw.buffer)
	return rw
}
