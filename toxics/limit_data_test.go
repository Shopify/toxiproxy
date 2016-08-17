package toxics_test

import (
	"math/rand"
	"testing"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
)

func cmpBuffers(bufa []byte, bufb []byte) bool {
	if len(bufa) != len(bufb) {
		return false
	}

	for i, a := range bufa {
		if a != bufb[i] {
			return false
		}
	}

	return true
}

func buffer(size int) []byte {
	buf := make([]byte, size)

	for i := 0; i < size; i++ {
		buf[i] = byte(rand.Int())
	}

	return buf
}

func check(t *testing.T, toxic *toxics.LimitDataToxic, chunks [][]byte, expectedChunks [][]byte) {
	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk, 100)
	stub := toxics.NewToxicStub(input, output)

	go func() {
		toxic.Pipe(stub)
	}()

	for _, buf := range chunks {
		input <- &stream.StreamChunk{Data: buf}
	}

	for _, expected := range expectedChunks {
		chunk := <-output

		if !cmpBuffers(chunk.Data, expected) {
			t.Fail()
		}
	}

	if len(output) != 0 {
		t.Fail()
	}
}

func TestLimitDataToxicMayBeInterrupted(t *testing.T) {
	toxic := &toxics.LimitDataToxic{Bytes: 100}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)

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

	buf := buffer(100)
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
