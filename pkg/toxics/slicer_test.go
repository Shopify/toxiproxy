package toxics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/pkg/stream"
)

func TestSlicerToxic(t *testing.T) {
	data := []byte(strings.Repeat("hello world ", 40000)) // 480 kb
	slicer := &Slicer{AverageSize: 1024, SizeVariation: 512, Delay: 10}

	input := make(chan *stream.Chunk)
	output := make(chan *stream.Chunk)
	stub := NewStub(input, output)

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

	input <- &stream.Chunk{Data: data}

	buf := make([]byte, 0, len(data))
	reads := 0
L:
	for {
		select {
		case c := <-output:
			reads++
			buf = append(buf, c.Data...)
		case <-time.After(10 * time.Millisecond):
			break L
		}
	}

	if reads < 480/2 || reads > 480/2+480 {
		t.Errorf("Expected to read about 480 times, but read %d times.", reads)
	}
	if bytes.Compare(buf, data) != 0 {
		t.Errorf("Server did not read correct buffer from client!")
	}
}
