package main

import (
	"fmt"
	"io"
	"math/rand"
)

// Implements the io.WriteCloser interface for a chan []byte
type ChanWriter chan<- []byte

func NewChanWriter(output chan<- []byte) ChanWriter {
	return ChanWriter(output)
}

func (c ChanWriter) Write(buf []byte) (int, error) {
	buf2 := make([]byte, len(buf))
	copy(buf2, buf) // Make a copy before sending it to the channel
	c <- buf2
	return len(buf), nil
}

func (c ChanWriter) Close() error {
	close(c)
	return nil
}

// Implements the io.Reader interface for a chan []byte
type ChanReader <-chan []byte

func NewChanReader(input <-chan []byte) ChanReader {
	return ChanReader(input)
}

func (c ChanReader) Read(buf []byte) (int, error) {
	buf2 := <-c
	if buf2 == nil {
		return 0, io.EOF
	}
	n := copy(buf, buf2)
	return n, nil
}

// PacketizeCopy() breaks up the input stream into random packets of size 64-32k bytes.
// This copy function is a modified version of io.Copy()
func PacketizeCopy(dst io.WriteCloser, src io.Reader) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf[0 : rand.Intn(len(buf)+64)+64]) // Random packet sizes of 64 - 32k bytes
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = fmt.Errorf("Write error: %v", ew)
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
			err = fmt.Errorf("Read error: %v", er)
			break
		}
	}
	return written, err
}
