package main

import (
	"math/rand"
	"time"
)

// The SlicerToxic slices data into multiple smaller packets
// to simulate real-world TCP behaviour.
type SlicerToxic struct {
	Enabled bool `json:"enabled"`
	// Average number of bytes to slice at
	AverageSize int `json:"average_size"`
	// +/- bytes to vary sliced amounts. Must be less than
	// the average size
	SizeVariation int `json:"size_variation"`
	// Microseconds to delay each packet. May be useful since there's
	// usually some kind of buffering of network data
	Delay int `json:"delay"`
}

func (t *SlicerToxic) Name() string {
	return "slicer"
}

func (t *SlicerToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *SlicerToxic) SetEnabled(enabled bool) {
	t.Enabled = enabled
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
		case <-stub.interrupt:
			return
		case c := <-stub.input:
			if c == nil {
				stub.Close()
				return
			}

			chunks := t.chunk(0, len(c.data))
			for i := 1; i < len(chunks); i += 2 {
				stub.output <- &StreamChunk{
					data:      c.data[chunks[i-1]:chunks[i]],
					timestamp: c.timestamp,
				}

				select {
				case <-stub.interrupt:
					stub.output <- &StreamChunk{
						data:      c.data[chunks[i]:],
						timestamp: c.timestamp,
					}
					return
				case <-time.After(time.Duration(t.Delay) * time.Microsecond):
				}
			}
		}
	}
}
