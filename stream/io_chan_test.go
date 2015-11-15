package stream

import (
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
