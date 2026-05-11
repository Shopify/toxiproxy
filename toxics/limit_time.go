package toxics

import "time"

// LimitTimeToxic has shuts connection after given time
type LimitTimeToxic struct {
	Time int64 `json:"time"`
}

type LimitTimeToxicState struct {
	ElapsedMilliseconds int64
}

func (t *LimitTimeToxic) Pipe(stub *ToxicStub) {
	state := stub.State.(*LimitTimeToxicState)

	if state.ElapsedMilliseconds >= t.Time {
		stub.Close()
		return
	}

	timeout := time.Duration(t.Time-state.ElapsedMilliseconds) * time.Millisecond
	start := time.Now()
	for {
		select {
		case <-time.After(timeout):
			stub.Close()
			return
		case <-stub.Interrupt:
			state.ElapsedMilliseconds = int64(time.Since(start) / time.Millisecond)
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}
			stub.Output <- c
		}
	}
}

func (t *LimitTimeToxic) NewState() interface{} {
	return new(LimitTimeToxicState)
}

func init() {
	Register("limit_time", new(LimitTimeToxic))
}
