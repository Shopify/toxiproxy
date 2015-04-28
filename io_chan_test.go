package main

import (
	"bytes"
	"io"
	"testing"
)

func TestBasicReadWrite(t *testing.T) {
	send := []byte("hello world")
	c := make(chan *Packet)
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
	c := make(chan *Packet)
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
	c := make(chan *Packet)
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
	if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err)
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
	c := make(chan *Packet)
	writer := NewChanWriter(c)
	reader := NewChanReader(c)
	go func() {
		writer.Write(send[:9])
		writer.Write(send[9:19])
		writer.Write(send[19:])
		writer.Close()
	}()
	buf := make([]byte, 10)
	read := 0
	for i := 0; i < len(send)/10; i++ {
		n, err := reader.Read(buf)
		if err != nil {
			t.Fatal("Couldn't read from stream", err)
		}
		if !bytes.Equal(buf[:n], send[read:read+n]) {
			t.Fatal("Got wrong message from stream", string(buf))
		}
		read += n
	}
	n, err := reader.Read(buf)
	if err != io.EOF {
		t.Fatal("Read returned wrong error after close:", err)
	}
	if !bytes.Equal(buf[:n], send[len(send)-n:]) {
		t.Fatal("Got wrong message from stream", string(buf[:n]))
	}
}

func TestMultiWriteWithCopy(t *testing.T) {
	send := []byte("hello world, this message is longer")
	c := make(chan *Packet)
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
