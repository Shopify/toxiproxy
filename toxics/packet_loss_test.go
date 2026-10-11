package toxics_test

import (
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

// TestPacketLossToxicNoLoss verifies that with loss_rate=0 every chunk passes.
func TestPacketLossToxicNoLoss(t *testing.T) {
	toxic := &toxics.PacketLossToxic{LossRate: 0.0, Correlation: 0.0}
	runPipeTest(t, toxic, 1000, 0)
}

// TestPacketLossToxicFullLoss verifies that with loss_rate=1 no chunk passes.
func TestPacketLossToxicFullLoss(t *testing.T) {
	toxic := &toxics.PacketLossToxic{LossRate: 1.0, Correlation: 0.0}
	runPipeTest(t, toxic, 100, 100)
}

// TestPacketLossToxicApproximateRate verifies drop rate is within ±10% of target.
func TestPacketLossToxicApproximateRate(t *testing.T) {
	const targetRate = 0.20
	toxic := &toxics.PacketLossToxic{LossRate: targetRate, Correlation: 0.0}

	const n = 1000
	dropped, _ := runPipeTestWithStats(t, toxic, n)

	empirical := float64(dropped) / float64(n)
	tolerance := 0.10

	if empirical < targetRate-tolerance || empirical > targetRate+tolerance {
		t.Errorf("empirical drop rate %.2f outside [%.2f, %.2f]",
			empirical, targetRate-tolerance, targetRate+tolerance)
	}
}

// TestPacketLossToxicIndependentConnections verifies independent RNG per connection.
func TestPacketLossToxicIndependentConnections(t *testing.T) {
	toxic := &toxics.PacketLossToxic{LossRate: 0.5}
	s1 := toxic.NewState()
	s2 := toxic.NewState()

	if s1 == s2 {
		t.Error("NewState must return different instances per connection")
	}
}

// TestPacketLossToxicPipeDrains confirms Pipe exits cleanly on interrupt.
func TestPacketLossToxicPipeDrains(t *testing.T) {
	toxic := &toxics.PacketLossToxic{LossRate: 0.0}

	input := make(chan *stream.StreamChunk, 1)
	output := make(chan *stream.StreamChunk, 1)

	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	done := make(chan struct{})
	go func() {
		toxic.Pipe(stub)
		close(done)
	}()

	close(stub.Interrupt)

	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("Pipe did not exit within 1s after interrupt")
	}
}

// runPipeTest sends n chunks and asserts exactly expectDropped were dropped.
func runPipeTest(t *testing.T, toxic *toxics.PacketLossToxic, n int, expectDropped int) {
	t.Helper()
	dropped, passed := runPipeTestWithStats(t, toxic, n)
	if dropped != expectDropped {
		t.Errorf("expected %d dropped, got %d (passed: %d)", expectDropped, dropped, passed)
	}
}

// runPipeTestWithStats returns (dropped, passed) counts.
// It drains output concurrently to avoid blocking the pipe.
func runPipeTestWithStats(t *testing.T, toxic *toxics.PacketLossToxic, n int) (int, int) {
	t.Helper()

	// Buffer input generously so the sender never blocks.
	input := make(chan *stream.StreamChunk, n)
	// Unbuffered output — drained by a goroutine below.
	output := make(chan *stream.StreamChunk)

	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	// Fill input before starting Pipe so timing doesn't matter.
	for i := 0; i < n; i++ {
		input <- &stream.StreamChunk{
			Data:      []byte{byte(i % 256)},
			Timestamp: time.Now(),
		}
	}
	close(input)

	// Drain output concurrently so Pipe never blocks on send.
	passed := 0
	drainDone := make(chan struct{})
	go func() {
		for range output {
			passed++
		}
		close(drainDone)
	}()

	// Run Pipe; it exits and closes output when input is closed.
	toxic.Pipe(stub)

	<-drainDone

	dropped := n - passed
	return dropped, passed
}
