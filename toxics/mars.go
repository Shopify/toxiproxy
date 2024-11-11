package toxics

import (
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/Shopify/toxiproxy/v2/stream"
)

// The MarsToxic simulates the communication delay to Mars based on current orbital positions.
// There are more accurate orbital models, but this is a simple approximation.
// 
// Further possibilities here:
//   * drop packets entirely during solar conjunction
//   * corrupt frames in the liminal period before/after conjunction
//   * buffering through the disk (maybe a FIFO, idk) would model data in flight better
//
// We could to the hard block but we're kind of at the wrong layer to do corruption.
type MarsToxic struct {
	// Optional additional latency in milliseconds
	ExtraLatency int64 `json:"extra_latency"`
	// Rate in KB/s (0 means unlimited)
	Rate int64 `json:"rate"`
	// Reference time for testing, if zero current time is used
	ReferenceTime time.Time `json:"-"`
	// Speed of light in km/s (defaults to 299792.458 if 0) It's (probably?)
	// obvious you won't want to change this. It's useful for testing.
	SpeedOfLight float64 `json:"speed_of_light"`
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
	
	// Speed of light is exactly 299,792.458 km/s by default
	speedOfLight := t.SpeedOfLight
	if speedOfLight <= 0 {
		speedOfLight = 299792.458 // km/s
	}
	
	// One-way time = distance / speed of light
	// Convert to milliseconds
	delayMs := int64((distanceKm / speedOfLight) * 1000)
	
	// Add any extra latency specified
	delayMs += t.ExtraLatency
	
	return time.Duration(delayMs) * time.Millisecond
}

func (t *MarsToxic) Pipe(stub *ToxicStub) {
	logger := log.With().
		Str("component", "MarsToxic").
		Str("method", "Pipe").
		Str("toxic_type", "mars").
		Str("addr", fmt.Sprintf("%p", t)).
		Logger()

	var sleep time.Duration = 0
	for {
		select {
		case <-stub.Interrupt:
			logger.Trace().Msg("MarsToxic was interrupted")
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}

			// Set timestamp when we receive the chunk
			if c.Timestamp.IsZero() {
				c.Timestamp = time.Now()
			}

			// Calculate Mars delay once for this chunk
			marsDelay := t.Delay()

			// Calculate bandwidth delay if rate is set
			if t.Rate > 0 {
				bytesPerSecond := t.Rate * 1024
				
				// If chunk is too large, split it
				if int64(len(c.Data)) > bytesPerSecond/10 { // 100ms worth of data
					bytesPerInterval := bytesPerSecond/10 // bytes per 100ms
					remainingData := c.Data
					chunkStart := c.Timestamp
					
					// First, wait for Mars delay
					select {
					case <-time.After(marsDelay):
					case <-stub.Interrupt:
						return
					}
					
					for len(remainingData) > 0 {
						chunkSize := int(bytesPerInterval)
						if chunkSize > len(remainingData) {
								chunkSize = len(remainingData)
						}
						
						chunk := &stream.StreamChunk{
								Data:      remainingData[:chunkSize],
								Timestamp: chunkStart,
						}
						
						select {
						case <-time.After(100 * time.Millisecond):
							chunkStart = chunkStart.Add(100 * time.Millisecond)
							stub.Output <- chunk
							remainingData = remainingData[chunkSize:]
						case <-stub.Interrupt:
							logger.Trace().Msg("MarsToxic was interrupted during writing data")
							return
						}
					}
					continue
				}
				
				// For small chunks, calculate bandwidth delay
				sleep = time.Duration(float64(len(c.Data)) / float64(bytesPerSecond) * float64(time.Second))
			}

			// Apply both Mars delay and bandwidth delay
			totalDelay := marsDelay
			if sleep > 0 {
				totalDelay += sleep
			}

			select {
			case <-time.After(totalDelay):
				c.Timestamp = c.Timestamp.Add(totalDelay)
				stub.Output <- c
			case <-stub.Interrupt:
				logger.Trace().Msg("MarsToxic was interrupted during writing data")
				err := stub.WriteOutput(c, 5*time.Second)
				if err != nil {
					logger.Warn().Err(err).Msg("Could not write last packets after interrupt")
				}
				return
			}
		}
	}
}

func init() {
	Register("mars", new(MarsToxic))
} 