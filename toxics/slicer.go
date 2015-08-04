package toxics

import (
	"math/rand"
	"time"

	"github.com/Shopify/toxiproxy/stream"
)

// The SlicerToxic slices data into multiple smaller packets
// to simulate real-world TCP behaviour.
type SlicerToxic struct {
	// Average number of bytes to slice at
	AverageSize int `json:"average_size"`
	// +/- bytes to vary sliced amounts. Must be less than
	// the average size
	SizeVariation int `json:"size_variation"`
	// Microseconds to delay each packet. May be useful since there's
	// usually some kind of buffering of network data
	Delay int `json:"delay"`
}

// Returns a list of chunk offsets to slice up a packet of the
// given total size. For example, for a size of 100, output might be:
//
//     []int{0, 18, 18, 43, 43, 67, 67, 77, 77, 100}
//           ^---^  ^----^  ^----^  ^----^  ^-----^
//
// This tries to get fairly evenly-varying chunks (no tendency
// to have a small/large chunk at the start/end).
func (t *SlicerToxic) chunk(start int, end int) []int {
	// Base case:
	// If the size is within the random varation, _or already
	// less than the average size_, just return it.
	// Otherwise split the chunk in about two, and recurse.
	if (end-start)-t.AverageSize <= t.SizeVariation {
		return []int{start, end}
	}

	// +1 in the size variation to offset favoring of smaller
	// numbers by integer division
	mid := start + (end-start)/2 + (rand.Intn(t.SizeVariation*2) - t.SizeVariation) + rand.Intn(1)
	left := t.chunk(start, mid)
	right := t.chunk(mid, end)

	return append(left, right...)
}

func (t *SlicerToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}

			chunks := t.chunk(0, len(c.Data))
			for i := 1; i < len(chunks); i += 2 {
				stub.Output <- &stream.StreamChunk{
					Data:      c.Data[chunks[i-1]:chunks[i]],
					Timestamp: c.Timestamp,
				}

				select {
				case <-stub.Interrupt:
					stub.Output <- &stream.StreamChunk{
						Data:      c.Data[chunks[i]:],
						Timestamp: c.Timestamp,
					}
					return
				case <-time.After(time.Duration(t.Delay) * time.Microsecond):
				}
			}
		}
	}
}

func init() {
	Register("slicer", new(SlicerToxic))
}
