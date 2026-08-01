package toxics

import (
	"math/rand"
	"time"
)

// The LatencyToxic passes data through with the a delay of latency +/- jitter added.
type LatencyToxic struct {
	// Times in milliseconds
	Latency int64 `json:"latency"`
	Jitter  int64 `json:"jitter"`
}

func (t *LatencyToxic) GetBufferSize() int {
	return 1024
}

func (t *LatencyToxic) delay() time.Duration {
	// Delay = t.Latency +/- t.Jitter
	delay := t.Latency
	jitter := t.Jitter
	if jitter > 0 {
		// #nosec G404 -- was ignored before too
		delay += rand.Int63n(jitter*2) - jitter
	}
	return time.Duration(delay) * time.Millisecond
}

// ExpectedDelay reports the largest delay this toxic can add to a chunk.
func (t *LatencyToxic) ExpectedDelay() time.Duration {
	return time.Duration(t.Latency+t.Jitter) * time.Millisecond
}

func (t *LatencyToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}
			sleep := t.delay() - time.Since(c.Timestamp)
			select {
			case <-time.After(sleep):
				c.Timestamp = c.Timestamp.Add(sleep)
				_ = stub.WriteOutput(c, stub.Timeout())
			case <-stub.Interrupt:
				// Exit fast without applying latency, but still bounded: the
				// next toxic may already be gone.
				_ = stub.WriteOutput(c, stub.Timeout())
				return
			}
		}
	}
}

func init() {
	Register("latency", new(LatencyToxic))
}
