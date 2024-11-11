package toxics

import (
	"math"
	"time"
)

// The MarsToxic simulates the communication delay to Mars based on current orbital positions.
// There are more accurate orbital models, but this is a simple approximation.
// 
// Further possibilities here:
//   * drop packets entirely during solar conjunction
//   * corrupt frames in the liminal period before/after conjunction
//
// We could to the hard block but we're kind of at the wrong layer to do corruption.
type MarsToxic struct {
	// Optional additional latency in milliseconds
	ExtraLatency int64 `json:"extra_latency"`
	// Reference time for testing, if zero current time is used
	ReferenceTime time.Time `json:"-"`
}

// Since we're buffering for several minutes, we need a large buffer.
// Maybe this should really be unbounded... this is actually a kind of awkward thing to model without
// a, you know, hundred million kilometre long buffer of functionally infinite
// capacity connecting the two points.
func (t *MarsToxic) GetBufferSize() int {
	return 1024 * 1024
}

// This is accurate to within a something like 1-2%, and probably not
// hundreds/thousands of years away from 2000.
// This is a simple sinusoidal approximation; a real calculation would require
// quite a lot more doing (which could be fun, but...)
func (t *MarsToxic) Delay() time.Duration {
	// Constants for Mars distance calculation
	minDistance := 54.6e6    // km at opposition
	maxDistance := 401.0e6   // km at conjunction
	meanDistance := (maxDistance + minDistance) / 2
	amplitude := (maxDistance - minDistance) / 2
	synodicPeriod := 779.96  // More precise synodic period in days
	
	// Calculate days since Jan 1, 2000
	// July 27, 2018 was a recent opposition, which was day 6763 since Jan 1, 2000
	baseDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var daysSince2000 float64
	if !t.ReferenceTime.IsZero() {
		daysSince2000 = t.ReferenceTime.Sub(baseDate).Hours() / 24
	} else {
		daysSince2000 = time.Since(baseDate).Hours() / 24
	}
	
	// Calculate phase based on synodic period
	phase := 2 * math.Pi * math.Mod(daysSince2000-6763, synodicPeriod) / synodicPeriod
	
	// Calculate current distance in kilometers
	distanceKm := meanDistance - amplitude*math.Cos(phase)
	
	// Speed of light is exactly 299,792.458 km/s
	speedOfLight := 299792.458 // km/s
	
	// One-way time = distance / speed of light
	// Convert to milliseconds
	delayMs := int64((distanceKm / speedOfLight) * 1000)
	
	// Add any extra latency specified
	delayMs += t.ExtraLatency
	
	return time.Duration(delayMs) * time.Millisecond
}

func (t *MarsToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}
			sleep := t.Delay() - time.Since(c.Timestamp)
			select {
			case <-time.After(sleep):
				c.Timestamp = c.Timestamp.Add(sleep)
					stub.Output <- c
			case <-stub.Interrupt:
				// Exit fast without applying latency.
				stub.Output <- c // Don't drop any data on the floor
				return
			}
		}
	}
}

func init() {
	Register("mars", new(MarsToxic))
} 