package toxics_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

// Every attribute combination must slice a packet into pieces that reassemble
// exactly, including the zero defaults `toxiproxy-cli toxic add -t slicer`
// sends and a size_variation larger than average_size.
func TestSlicerReassemblesForAnyAttributes(t *testing.T) {
	params := []struct{ average, variation int }{
		{0, 0}, {0, 5}, {-1, 0}, {1, 0}, {2, 1}, {5, 10}, {10, 10}, {64, 8}, {1024, 512},
	}
	sizes := []int{1, 2, 21, 64, 1000, 4096}

	for _, p := range params {
		for _, size := range sizes {
			toxic := &toxics.SlicerToxic{AverageSize: p.average, SizeVariation: p.variation}
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i % 251)
			}

			input := make(chan *stream.StreamChunk, 1)
			output := make(chan *stream.StreamChunk, size+16)
			stub := toxics.NewToxicStub(input, output)

			done := make(chan []byte, 1)
			go func() {
				var got bytes.Buffer
				for chunk := range output {
					got.Write(chunk.Data)
				}
				done <- got.Bytes()
			}()

			go toxic.Pipe(stub)
			input <- &stream.StreamChunk{Data: payload, Timestamp: time.Now()}
			close(input)

			select {
			case got := <-done:
				if !bytes.Equal(got, payload) {
					t.Errorf("params %+v size %d: reassembled %d bytes, want %d", p, size, len(got), size)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("params %+v size %d: no output within 5s", p, size)
			}
		}
	}
}
