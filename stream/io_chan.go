package stream

import (
	"bytes"
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
	if len(buf) == 0 {
		return 0, nil
	}

	packet := &StreamChunk{make([]byte, len(buf)), time.Now()}
	copy(packet.Data, buf) // Make a copy before sending it to the channel
	c.output <- packet
	return len(buf), nil
}

func (c *ChanWriter) SetOutput(output chan<- *StreamChunk) {
	c.output = output
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

// TransactionalReader is a ChanReader that can rollback its progress to checkpoints.
// This is useful when using other buffered readers, since they may read past the end of a message.
// The buffered reader can later be removed by rolling back any buffered bytes.
//
// chan []byte ->  ChanReader -> TeeReader  ->    Read()   -> output
//                                   V              ^
//                             bytes.Buffer -> bytes.Reader
type TransactionalReader struct {
	buffer    *bytes.Buffer
	bufReader *bytes.Reader
	reader    *ChanReader
	tee       io.Reader
}

func NewTransactionalReader(input <-chan *StreamChunk) *TransactionalReader {
	t := &TransactionalReader{
		buffer: bytes.NewBuffer(make([]byte, 0, 32*1024)),
		reader: NewChanReader(input),
	}
	t.tee = io.TeeReader(t.reader, t.buffer)
	return t
}

// Reads from the input channel either directly, or from a buffer if Rollback() has been called.
// If the reader returns `ErrInterrupted`, it will automatically call Rollback()
func (t *TransactionalReader) Read(out []byte) (n int, err error) {
	defer func() {
		if err == ErrInterrupted || err == io.EOF {
			t.Rollback()
		}
	}()

	if t.bufReader != nil {
		n, err := t.bufReader.Read(out)
		if err == io.EOF {
			t.bufReader = nil
			if n > 0 {
				return n, nil
			} else {
				return t.tee.Read(out)
			}
		}
		return n, err
	} else {
		return t.tee.Read(out)
	}
}

// Flushes all buffers past the current position in the reader to the specified writer.
func (t *TransactionalReader) FlushTo(writer io.Writer) {
	n := 0
	if t.bufReader != nil {
		n = t.bufReader.Len()
	}
	buf := make([]byte, n+len(t.reader.buffer))
	if n > 0 {
		t.bufReader.Read(buf[:n])
	}
	if len(buf[n:]) > 0 {
		t.reader.Read(buf[n:])
	}
	writer.Write(buf)
	t.bufReader = nil
	t.buffer.Reset()
}

// Sets a checkpoint in the reader. A call to Rollback() will begin reading from this point.
// If offset is negative, the checkpoint will be set N bytes before the current position.
// If the offset is positive, the checkpoint will be set N bytes after the previous checkpoint.
// An offset of 0 will set the checkpoint to the current position.
func (t *TransactionalReader) Checkpoint(offset int) {
	current := t.buffer.Len()
	if t.bufReader != nil {
		current = int(t.bufReader.Size()) - t.bufReader.Len()
	}

	n := current
	if offset > 0 {
		n = offset
	} else {
		n = current + offset
	}

	if n >= current {
		t.buffer.Reset()
	} else {
		t.buffer.Next(n)
	}
}

// Rolls back the reader to start from the last checkpoint.
func (t *TransactionalReader) Rollback() {
	t.bufReader = bytes.NewReader(t.buffer.Bytes())
}

func (t *TransactionalReader) SetInterrupt(interrupt <-chan struct{}) {
	t.reader.SetInterrupt(interrupt)
}
