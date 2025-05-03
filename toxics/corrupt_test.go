package toxics_test

import (
	"strings"
	"testing"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func count_flips(before, after []byte) int {
	res := 0
	for i := 0; i < len(before); i++ {
		if before[i] != after[i] {
			res += 1
		}
	}
	return res
}

func DoCorruptEcho(corrupt *toxics.CorruptToxic) ([]byte, []byte) {
	len_data := 100
	data0 := []byte(strings.Repeat("a", len_data))
	data1 := make([]byte, len_data)
	copy(data1, data0)

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)

	done := make(chan bool)
	go func() {
		corrupt.Pipe(stub)
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

	input <- &stream.StreamChunk{Data: data1}

	result := <-output
	return data0, result.Data
}

func TestCorruptToxicLowProb(t *testing.T) {
	corrupt := &toxics.CorruptToxic{Prob: 0.001}
	original, corrupted := DoCorruptEcho(corrupt)

	num_flips := count_flips(original, corrupted)

	tolerance := 5
	expected := 0
	if num_flips > expected+tolerance {
		t.Errorf("Too many bytes flipped! (note: this test has a very low false positive probability)")
	}
}

func TestCorruptToxicHighProb(t *testing.T) {
	corrupt := &toxics.CorruptToxic{Prob: 0.999}
	original, corrupted := DoCorruptEcho(corrupt)

	num_flips := count_flips(original, corrupted)

	tolerance := 5
	expected := 100
	if num_flips < expected-tolerance {
		t.Errorf("Too few bytes flipped! (note: this test has a very low false positive probability)")
	}
}
