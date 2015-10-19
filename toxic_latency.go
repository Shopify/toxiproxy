package main

import (
	"math/rand"
	"time"
)

// The LatencyToxic passes data through with the a delay of latency +/- jitter added.
type LatencyToxic struct {
	Enabled bool `json:"enabled"`
	// Times in milliseconds
	Latency int64 `json:"latency"`
	Jitter  int64 `json:"jitter"`
	// Percent of requests that will be passed without any effect of this latency feature.
	// Between 0-100 . 
	Skip int32 `json:"skip"`
}

func (t *LatencyToxic) Name() string {
	return "latency"
}

func (t *LatencyToxic) IsEnabled() bool {
	return t.Enabled
}

func (t *LatencyToxic) SetEnabled(enabled bool) {
	t.Enabled = enabled
}

func (t *LatencyToxic) delay() time.Duration {
	// Delay = t.Latency +/- t.Jitter
	delay := t.Latency
	jitter := int64(t.Jitter)
	
	// Percent of messages that will be passed without any effect of this latency feature.
	// When not set, skip = 0 by default, all messages are affected as usual.
	skip := int32(t.Skip)
	
	if jitter > 0 {
		delay += rand.Int63n(jitter*2) - jitter
	}

	// Do not apply this latency to this percent of packages.
	if(skip>0){

		if(skip>100) { // do not allow percentages bigger than 100
			skip = 100
			logrus.WithFields(logrus.Fields{
				"skip":  skip,
			}).Warn("Attempted to set a skip value > 100. Resetting to 100.")
		}

		var randValue = rand.Int31n(100);
		if((randValue+1) <= skip){
			delay = 0
		}
	} else if (skip <0) {
		logrus.WithFields(logrus.Fields{
				"skip":  skip,
			}).Warn("Attempted to set a skip value < 0. Ignoring this value.")
	}
	
	return time.Duration(delay) * time.Millisecond
}

func (t *LatencyToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.interrupt:
			return
		case c := <-stub.input:
			if c == nil {
				stub.Close()
				return
			}
			sleep := t.delay() - time.Now().Sub(c.timestamp)
			select {
			case <-time.After(sleep):
				stub.output <- c
			case <-stub.interrupt:
				stub.output <- c // Don't drop any data on the floor
				return
			}
		}
	}
}
