package toxics
 
// PacketLossToxic randomly drops StreamChunks passing through the proxy,
// simulating packet loss / flaky network conditions.
//
// Attributes:
//   - loss_rate : float64  probability [0.0–1.0] that a chunk is dropped
//                          (default 0.1 -> 10 %)
//   - correlation : float64  probability [0.0–1.0] that the *next* chunk is
//                          also dropped when the previous one was (Gilbert-
//                          Elliott burst model; default 0.0 -> no burstiness)
//
// How it fits toxiproxy's pipeline:
//
//	Client -> [noop] -> [packet_loss] -> [noop] -> Upstream
//	                         |
//	                  dropped chunks
//	                   are discarded
//
// Registration happens automatically via init().
 
import (
	"math/rand"
)
 
// PacketLossToxicState holds per-connection mutable state so that the
// main toxic struct (shared across connections) stays read-only.
type PacketLossToxicState struct {
	// wasDropped records whether the previous chunk was dropped; used for
	// burst-correlation logic.
	wasDropped bool
	// rng is a per-connection source so concurrent connections don't
	// contend on a shared global rand.
	rng *rand.Rand
}
 
// PacketLossToxic is the toxic struct. Fields are JSON-tagged to match
// toxiproxy's HTTP API convention.
type PacketLossToxic struct {
	// LossRate is the baseline probability that any individual chunk is
	// dropped. Range [0.0, 1.0]. Default 0.1 (10 %).
	LossRate float64 `json:"loss_rate"`
 
	// Correlation is the extra probability that the *next* chunk is dropped
	// when the current one was dropped, modelling burst packet loss (Gilbert-
	// Elliott model simplified). Range [0.0, 1.0]. Default 0.0.
	Correlation float64 `json:"correlation"`
}
 
// NewState satisfies the StatefulToxic interface. toxiproxy calls this once
// per new connection so every connection gets its own RNG and drop state.
func (t *PacketLossToxic) NewState() interface{} {
	return &PacketLossToxicState{
		rng: rand.New(rand.NewSource(rand.Int63())),
	}
}
 
// Pipe satisfies the Toxic interface. It reads chunks from stub.Input,
// decides whether to forward or drop each one, and writes survivors to
// stub.Output. It exits when the input channel is closed or an interrupt
// arrives.
func (t *PacketLossToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*PacketLossToxicState)
 
	// Clamp configuration to valid ranges once, up front.
	lossRate := clamp(t.LossRate, 0.0, 1.0)
	correlation := clamp(t.Correlation, 0.0, 1.0)
 
	for {
		select {
		case <-stub.Interrupt:
			// toxiproxy is removing or reconfiguring this toxic; drain cleanly.
			return
 
		case chunk, ok := <-stub.Input:
			if !ok {
				// Upstream closed the connection.
				return
			}
 
			if t.shouldDrop(state, lossRate, correlation) {
				// Drop: discard the chunk entirely. The byte slice is simply
				// not forwarded – no close, no RST – mimicking a lost IP packet.
				state.wasDropped = true
				continue
			}
 
			state.wasDropped = false
 
			// Forward the chunk unmodified.
			select {
			case stub.Output <- chunk:
			case <-stub.Interrupt:
				return
			}
		}
	}
}
 
// shouldDrop returns true when the current chunk must be discarded.
// It implements a simplified Gilbert-Elliott two-state model:
//   - In the "good" state  -> drop with probability lossRate
//   - In the "bad" state   -> drop with probability (lossRate + correlation),
//     where "bad" means the previous chunk was also dropped.
func (t *PacketLossToxic) shouldDrop(
	state *PacketLossToxicState,
	lossRate, correlation float64,
) bool {
	p := lossRate
	if state.wasDropped {
		p = min(1.0, lossRate+correlation)
	}
	return state.rng.Float64() < p
}
 
// init registers the toxic with toxiproxy automatically at startup.
func init() {
	Register("packet_loss", new(PacketLossToxic))
}
 
// clamp constrains v to the range [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}