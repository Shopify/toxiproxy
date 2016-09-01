package toxics_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
)

func buffer(size int) []byte {
	buf := make([]byte, size)
	rand.Read(buf)

	return buf
}

func checkOutgoingChunk(t *testing.T, output chan *stream.StreamChunk, expected []byte) {
	chunk := <-output
	if !bytes.Equal(chunk.Data, expected) {
		t.Error("Data in outgoing chunk doesn't match expected values")
	}
}

func checkRemainingChunks(t *testing.T, output chan *stream.StreamChunk) {
	if len(output) != 0 {
		t.Error(fmt.Sprintf("There is %d chunks in output channel. 0 is expected.", len(output)))
	}
}

func check(t *testing.T, toxic *toxics.LimitDataToxic, chunks [][]byte, expectedChunks [][]byte) {
	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk, 100)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	go toxic.Pipe(stub)

	for _, buf := range chunks {
		input <- &stream.StreamChunk{Data: buf}
	}

	for _, expected := range expectedChunks {
		checkOutgoingChunk(t, output, expected)
	}

	checkRemainingChunks(t, output)
}

func TestLimitDataToxicMayBeRestarted(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk, 100)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	buf := buffer(90)
	buf2 := buffer(20)

	// Send chunk with data not exceeding limit and interrupt
	go func() {
		input <- &stream.StreamChunk{Data: buf}
		stub.Interrupt <- struct{}{}
	}()

	toxic.Pipe(stub)
	checkOutgoingChunk(t, output, buf)

	// Send 2nd chunk to exceed limit
	go func() {
		input <- &stream.StreamChunk{Data: buf2}
	}()

	toxic.Pipe(stub)
	checkOutgoingChunk(t, output, buf2[0:10])

	checkRemainingChunks(t, output)
}

func TestLimitDataToxicMayBeInterrupted(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	go func() {
		stub.Interrupt <- struct{}{}
	}()

	toxic.Pipe(stub)
}

func TestLimitDataToxicNilShouldClosePipe(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	go func() {
		input <- nil
	}()

	toxic.Pipe(stub)
}

func TestLimitDataToxicChunkSmallerThanLimit(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	buf := buffer(50)
	check(t, toxic, [][]byte{buf}, [][]byte{buf})
}

func TestLimitDataToxicChunkLengthMatchesLimit(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	buf := buffer(100)
	check(t, toxic, [][]byte{buf}, [][]byte{buf})
}

func TestLimitDataToxicChunkBiggerThanLimit(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	buf := buffer(150)
	expected := buf[0:100]

	check(t, toxic, [][]byte{buf}, [][]byte{expected})
}

func TestLimitDataToxicMultipleChunksMatchThanLimit(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	buf := buffer(25)

	check(t, toxic, [][]byte{buf, buf, buf, buf}, [][]byte{buf, buf, buf, buf})
}

func TestLimitDataToxicSecondChunkWouldOverflowLimit(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	buf := buffer(90)
	buf2 := buffer(20)
	expected := buf2[0:10]

	check(t, toxic, [][]byte{buf, buf2}, [][]byte{buf, expected})
}

func TestLimitDataToxicLimitIsSetToZero(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 0}

	buf := buffer(100)

	check(t, toxic, [][]byte{buf}, [][]byte{})
}
