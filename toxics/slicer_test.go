package toxics_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func TestSlicerToxic(t *testing.T) {
	data := []byte(strings.Repeat("hello world ", 40000)) // 480 kb
	slicer := &toxics.SlicerToxic{AverageSize: 1024, SizeVariation: 512, Delay: 10}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)

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
	stub := toxics.NewToxicStub(input, output)

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

func TestSlicerToxicDegenerateAttributesTerminate(t *testing.T) {
	data := []byte(strings.Repeat("hello world ", 20))

	testCases := []struct {
		name   string
		slicer *toxics.SlicerToxic
	}{
		{"zero average size", &toxics.SlicerToxic{}},
		{"negative size variation", &toxics.SlicerToxic{AverageSize: 4, SizeVariation: -8}},
		{"size variation above average size", &toxics.SlicerToxic{AverageSize: 1, SizeVariation: 200}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := make(chan *stream.StreamChunk)
			output := make(chan *stream.StreamChunk)
			stub := toxics.NewToxicStub(input, output)

			go tc.slicer.Pipe(stub)
			input <- &stream.StreamChunk{Data: data}
			close(input)

			buf := make([]byte, 0, len(data))
			for c := range output {
				buf = append(buf, c.Data...)
			}

			if !bytes.Equal(buf, data) {
				t.Errorf("got %q; expected %q", buf, data)
			}
		})
	}
}

func TestSlicerToxicValidate(t *testing.T) {
	testCases := []struct {
		name    string
		slicer  *toxics.SlicerToxic
		isValid bool
	}{
		{"zero values", &toxics.SlicerToxic{}, false},
		{"negative average size", &toxics.SlicerToxic{AverageSize: -1}, false},
		{"negative size variation", &toxics.SlicerToxic{AverageSize: 10, SizeVariation: -1}, false},
		{"variation equal to average", &toxics.SlicerToxic{AverageSize: 10, SizeVariation: 10}, false},
		{"zero size variation", &toxics.SlicerToxic{AverageSize: 1}, true},
		{"variation below average", &toxics.SlicerToxic{AverageSize: 10, SizeVariation: 9}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.slicer.Validate()
			if tc.isValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tc.isValid && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
