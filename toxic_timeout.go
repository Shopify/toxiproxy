package main

import (
	"time"
	"math/rand"
)

// The TimeoutToxic stops any data from flowing through, and will close the connection after a timeout.
// If the timeout is set to 0, then the connection will not be closed.
type TimeoutToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Timeout int64 `json:"timeout"`
	// Percent of requests that will be passed without any effect of this timeout feature.
	// Between 0-100 . 
	Skip int32 `json:"skip"`
}

func (t *TimeoutToxic) Name() string {
	return "timeout"
}

func (t *TimeoutToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *TimeoutToxic) SetEnabled(enabled bool) {
	t.Enabled = enabled
}

func (t *TimeoutToxic) Pipe(stub *ToxicStub) {


	timeout := time.Duration(t.Timeout) * time.Millisecond

	// Percent of messages that will be passed without any effect of this timeout feature.
	// When not set, skip = 0 by default, all messages are affected as usual.
	skip := int32(t.Skip)	
		
	// Do not apply this timeout to this percent of packages.
	if(skip>0){
		var randValue = rand.Int31n(100);
		if((randValue+1) <= skip){
			return
		}
	}
	
	if timeout > 0 {
		select {
		case <-time.After(timeout):
			stub.Close()
			return
		case <-stub.interrupt:
			return
		}
	} else {
		<-stub.interrupt
		return
	}
}
