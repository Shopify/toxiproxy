package stream

import (
	"bufio"
	"bytes"
	"io"
	"testing"
	"time"
)

func TestBasicReadWrite(t *testing.T) {
	send := []byte("hello world")
	c := make(chan *StreamChunk)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	go writer.Write(send)
	buf := make([]byte, len(send))
	n, err := reader.Read(buf)
	if n != len(send) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(send))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf, send) {
		t.Fatal("Got wrong message from stream", string(buf))
	}
	writer.Close()
	n, err = reader.Read(buf)
	if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err)
	}
	if n != 0 {
		t.Fatalf("Read still returned data after close: %d bytes", n)
	}
}

func TestReadMoreThanWrite(t *testing.T) {
	send := []byte("hello world")
	c := make(chan *StreamChunk)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	go writer.Write(send)
	buf := make([]byte, len(send)+10)
	n, err := reader.Read(buf)
	if n != len(send) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(send))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf[:n], send) {
		t.Fatal("Got wrong message from stream", string(buf[:n]))
	}
	writer.Close()
	n, err = reader.Read(buf)
	if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err)
	}
	if n != 0 {
		t.Fatalf("Read still returned data after close: %d bytes", n)
	}
}

func TestReadLessThanWrite(t *testing.T) {
	send := []byte("hello world")
	c := make(chan *StreamChunk)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	go writer.Write(send)
	buf := make([]byte, 6)
	n, err := reader.Read(buf)
	if n != len(buf) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(buf))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf, send[:len(buf)]) {
		t.Fatal("Got wrong message from stream", string(buf))
	}
	writer.Close()
	n, err = reader.Read(buf)
	if n != len(send)-len(buf) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(send)-len(buf))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf[:n], send[len(buf):]) {
		t.Fatal("Got wrong message from stream", string(buf[:n]))
	}
	n, err = reader.Read(buf)
	if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err)
	}
	if n != 0 {
		t.Fatalf("Read still returned data after close: %d bytes", n)
	}
}

func TestMultiReadWrite(t *testing.T) {
	send := []byte("hello world, this message is longer")
	c := make(chan *StreamChunk)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	go func() {
		writer.Write(send[:9])
		writer.Write(send[9:19])
		writer.Write(send[19:])
		writer.Close()
	}()
	buf := make([]byte, 10)
	for read := 0; read < len(send); {
		n, err := reader.Read(buf)
		if err != nil {
			t.Fatal("Couldn't read from stream", err, n)
		}
		if !bytes.Equal(buf[:n], send[read:read+n]) {
			t.Fatal("Got wrong message from stream", string(buf))
		}
		read += n
	}
	n, err := reader.Read(buf)
	if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err, string(buf[:n]))
	}
	if !bytes.Equal(buf[:n], send[len(send)-n:]) {
		t.Fatal("Got wrong message from stream", string(buf[:n]))
	}
}

func TestMultiWriteWithCopy(t *testing.T) {
	send := []byte("hello world, this message is longer")
	c := make(chan *StreamChunk)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	go func() {
		writer.Write(send[:9])
		writer.Write(send[9:19])
		writer.Write(send[19:])
		writer.Close()
	}()
	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, reader)
	if int(n) != len(send) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(send))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf.Bytes(), send) {
		t.Fatal("Got wrong message from stream", buf.String())
	}
}

func TestMultiRead(t *testing.T) {
	send := []byte("hello world")
	c := make(chan *StreamChunk)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	passed := make(chan bool)
	go func() {
		writer.Write(send)
		select {
		case c <- &StreamChunk{[]byte("garbage"), time.Now()}:
		case <-passed:
		}
		writer.Close()
	}()
	buf := make([]byte, len(send))

	n, err := reader.Read(buf[:8])
	if n != 8 {
		t.Fatalf("Read wrong number of bytes: %d expected 8", n)
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf[:8], send[:8]) {
		t.Fatal("Got wrong message from stream", string(buf[:8]))
	}
	time.Sleep(10 * time.Millisecond)

	n, err = reader.Read(buf[8:])
	if n != len(buf[8:]) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(buf[8:]))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf, send) {
		t.Fatal("Got wrong message from stream", string(buf))
	}

	passed <- true

	n, err = reader.Read(buf)
	if n != 0 {
		t.Fatalf("Read from channel occured when it shouldn't have: %s", string(buf[:n]))
	} else if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err)
	}
}

func TestReadInterrupt(t *testing.T) {
	send := []byte("hello world")
	c := make(chan *StreamChunk)
	interrupt := make(chan struct{})
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	reader.SetInterrupt(interrupt)
	go writer.Write(send)
	buf := make([]byte, len(send))
	n, err := reader.Read(buf)
	if n != len(send) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(send))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf, send) {
		t.Fatal("Got wrong message from stream", string(buf))
	}

	// Try interrupting the stream mid-read
	go func() {
		time.Sleep(50 * time.Millisecond)
		interrupt <- struct{}{}
	}()
	n, err = reader.Read(buf)
	if err != ErrInterrupted {
		t.Fatal("Read returned wrong error after interrupt:", err)
	}
	if n != 0 {
		t.Fatalf("Read still returned data after interrput: %d bytes", n)
	}

	// Try writing again after the channel was interrupted
	go writer.Write(send)
	n, err = reader.Read(buf)
	if n != len(send) {
		t.Fatalf("Read wrong number of bytes: %d expected %d", n, len(send))
	}
	if err != nil {
		t.Fatal("Couldn't read from stream", err)
	}
	if !bytes.Equal(buf, send) {
		t.Fatal("Got wrong message from stream", string(buf))
	}
}

func TestBlankWrite(t *testing.T) {
	c := make(chan *StreamChunk, 2)
	writer := NewChanWriter(c)
	writer.Write([]byte{})
	writer.Write(nil)
	writer.Close()

	for v := range c {
		t.Fatalf("Unexpected write to channel: %+v", v)
	}
}

type mockCloser chan struct{}

func (c mockCloser) Close() {
	close(c)
}

func NewTestReadWriter() (*ChanReadWriter, chan *StreamChunk, chan struct{}) {
	input := make(chan *StreamChunk, 2)
	output := make(chan *StreamChunk, 1)
	closer := make(mockCloser)
	rw := NewChanReadWriter(input, output, closer)
	input <- &StreamChunk{[]byte("hello world"), time.Now()}
	input <- &StreamChunk{[]byte("foobar"), time.Now()}
	close(input)
	return rw, output, closer
}

func AssertRead(t *testing.T, rw *ChanReadWriter, buf []byte, msg string, expectedErr error) (n int, err error, ret bool) {
	n, err = rw.Read(buf)
	ret = rw.HandleError(err)

	if n != len(msg) {
		t.Fatalf("Read wrong number of bytes: %d expected %d (%s expected %s)", n, len(msg), string(buf[:n]), msg)
	}
	if !bytes.Equal(buf[:n], []byte(msg)) {
		t.Fatalf("Got wrong message from stream: %s expected %s", string(buf[:n]), msg)
	}

	if err != expectedErr {
		t.Fatal("Unexpected error during read:", err)
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF || err == ErrInterrupted {
		if !ret {
			t.Fatal("HandleError() returned true without error")
		}
	} else {
		if ret {
			t.Fatal("HandleError() did not return true for error:", err)
		}
	}
	return
}

func AssertClosed(t *testing.T, output chan *StreamChunk, closer chan struct{}, expectedOutput []byte) {
	select {
	case msg := <-output:
		if expectedOutput == nil {
			t.Fatal("Unexpected message written to output channel:", string(msg.Data))
		} else if !bytes.Equal(msg.Data, expectedOutput) {
			t.Fatal("Wrong message written to output channel:", string(msg.Data), "expected", string(expectedOutput))
		}
	default:
		if expectedOutput != nil {
			t.Fatal("Expected message to be written to output channel:", string(expectedOutput))
		}
	}

	select {
	case <-closer:
	default:
		t.Fatal("Closer was not closed at end of stream")
	}
}

func TestReadWriterBasicFull(t *testing.T) {
	rw, output, closer := NewTestReadWriter()
	buf := make([]byte, 32)

	AssertRead(t, rw, buf, "hello world", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "foobar", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, nil)
}

func TestReadWriterCheckpointRollback(t *testing.T) {
	rw, output, closer := NewTestReadWriter()
	buf := make([]byte, 8)

	AssertRead(t, rw, buf, "hello wo", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "rldfooba", nil)
	rw.Rollback()
	AssertRead(t, rw, buf, "rldfooba", nil)
	rw.Checkpoint(-3)
	AssertRead(t, rw, buf, "r", nil)
	rw.Rollback()
	AssertRead(t, rw, buf, "obar", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, nil)
}

func TestReadWriterFlush(t *testing.T) {
	rw, output, closer := NewTestReadWriter()
	buf := make([]byte, 8)

	AssertRead(t, rw, buf, "hello wo", nil)
	rw.Flush()
	AssertRead(t, rw, buf, "foobar", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, []byte("rld"))
}

func TestReadWriterNoCheckpoint(t *testing.T) {
	rw, output, closer := NewTestReadWriter()
	buf := make([]byte, 32)

	AssertRead(t, rw, buf, "hello world", nil)
	AssertRead(t, rw, buf, "foobar", nil)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, []byte("hello worldfoobar"))
}

func TestReadWriterInterrupt(t *testing.T) {
	input := make(chan *StreamChunk, 1)
	output := make(chan *StreamChunk, 1)
	closer := make(mockCloser)
	rw := NewChanReadWriter(input, output, closer)
	input <- &StreamChunk{[]byte("hello world"), time.Now()}
	interrupt := make(chan struct{})
	rw.SetInterrupt(interrupt)
	buf := make([]byte, 32)

	AssertRead(t, rw, buf, "hello world", nil)
	rw.Checkpoint(0)
	go func() {
		interrupt <- struct{}{}
		input <- &StreamChunk{[]byte("foobar"), time.Now()}
		close(input)
	}()
	AssertRead(t, rw, buf, "", ErrInterrupted)
	AssertRead(t, rw, buf, "foobar", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, nil)
}

func TestReadWriterBufferedReader(t *testing.T) {
	rw, output, closer := NewTestReadWriter()
	buf := make([]byte, 32)
	reader := bufio.NewReader(rw)
	msg, err := reader.ReadString(' ')
	ret := rw.HandleError(err)

	if ret || err != nil {
		t.Fatal("Read failed through bufio.Reader:", err, ret)
	}
	if msg != "hello " {
		t.Fatal("Buffered reader read wrong message:", msg)
	}
	if reader.Buffered() != 5 {
		t.Fatal("Unexpected number of buffered bytes in reader:", reader.Buffered())
	}

	rw.Checkpoint(-reader.Buffered())
	rw.Rollback()

	AssertRead(t, rw, buf, "world", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "foobar", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, nil)
}

func TestReadWriterBufferedReaderAlternate(t *testing.T) {
	rw, output, closer := NewTestReadWriter()
	buf := make([]byte, 32)
	reader := bufio.NewReader(rw)
	msg, err := reader.ReadString(' ')
	ret := rw.HandleError(err)

	if ret || err != nil {
		t.Fatal("Read failed through bufio.Reader:", err, ret)
	}
	if msg != "hello " {
		t.Fatal("Buffered reader read wrong message:", msg)
	}
	if reader.Buffered() != 5 {
		t.Fatal("Unexpected number of buffered bytes in reader:", reader.Buffered())
	}

	rw.Checkpoint(len(msg))
	rw.Rollback()

	AssertRead(t, rw, buf, "world", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "foobar", nil)
	rw.Checkpoint(0)
	AssertRead(t, rw, buf, "", io.EOF)

	AssertClosed(t, output, closer, nil)
}
