package toxics_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/rs/zerolog"
)

func TestSlicerToxic(t *testing.T) {
	data := []byte(strings.Repeat("hello world ", 40000)) // 480 kb
	slicer := &toxics.SlicerToxic{AverageSize: 1024, SizeVariation: 512, Delay: 10}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	logger := zerolog.Nop()
	stub := toxics.NewToxicStub(input, output, &logger)

	done := make(chan bool)
	go func() {
		slicer.Pipe(stub)
		done <- true
	}()
	defer func() {
		close(input)
		for {
			select {
			case <-done:
				return
			case <-output:
			}
		}
	}()

	input <- &stream.StreamChunk{Data: data}

	buf := make([]byte, 0, len(data))
	reads := 0

	for timeout := false; !timeout; {
		select {
		case c := <-output:
			reads++
			buf = append(buf, c.Data...)
		case <-time.After(10 * time.Millisecond):
			timeout = true
		}
	}

	if reads < 480/2 || reads > 480/2+480 {
		t.Errorf("Expected to read about 480 times, but read %d times.", reads)
	}
	if !bytes.Equal(buf, data) {
		t.Errorf("Server did not read correct buffer from client!")
	}
}

func TestSlicerToxicZeroSizeVariation(t *testing.T) {
	data := []byte(strings.Repeat("hello world ", 2)) // 24 bytes
	// SizeVariation: 0 by default
	slicer := &toxics.SlicerToxic{AverageSize: 1, Delay: 10}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	logger := zerolog.Nop()
	stub := toxics.NewToxicStub(input, output, &logger)

	done := make(chan bool)
	go func() {
		slicer.Pipe(stub)
		done <- true
	}()
	defer func() {
		close(input)
		for {
			select {
			case <-done:
				return
			case <-output:
			}
		}
	}()

	input <- &stream.StreamChunk{Data: data}

	buf := make([]byte, 0, len(data))
	reads := 0

	for timeout := false; !timeout; {
		select {
		case c := <-output:
			reads++
			buf = append(buf, c.Data...)
		case <-time.After(10 * time.Millisecond):
			timeout = true
		}
	}

	if reads != 24 {
		t.Errorf("Expected to read 24 times, but read %d times.", reads)
	}
	if !bytes.Equal(buf, data) {
		t.Errorf("Server did not read correct buffer from client!")
	}
}
