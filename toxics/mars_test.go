package toxics_test

import (
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/Shopify/toxiproxy/v2/stream"
)

func TestMarsDelayCalculation(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected time.Duration
	}{
		{
			name:     "At Opposition (Closest)",
			date:     time.Date(2018, 7, 27, 0, 0, 0, 0, time.UTC),
			expected: 182 * time.Second, // ~3 minutes at closest approach
		},
		{
			name:     "At Conjunction (Farthest)",
			date:     time.Date(2019, 9, 2, 0, 0, 0, 0, time.UTC), // ~400 days after opposition
			expected: 1337 * time.Second, // ~22.3 minutes at farthest point
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marsToxic := &toxics.MarsToxic{
				ReferenceTime: tt.date,
			}
			
			delay := marsToxic.Delay()
			tolerance := time.Duration(float64(tt.expected) * 0.04) // 4% tolerance
			if diff := delay - tt.expected; diff < -tolerance || diff > tolerance {
				t.Errorf("Expected delay of %v (±%v), got %v (%.1f%% difference)", 
					tt.expected, 
					tolerance, 
					delay,
					float64(diff) / float64(tt.expected) * 100,
				)
			}
		})
	}
}

func TestMarsExtraLatencyCalculation(t *testing.T) {
	marsToxic := &toxics.MarsToxic{
		ReferenceTime: time.Date(2018, 7, 27, 0, 0, 0, 0, time.UTC),
		ExtraLatency:  60000, // Add 1 minute
	}

	expected := 242 * time.Second // ~4 minutes (3 min base + 1 min extra)
	delay := marsToxic.Delay()
	
	tolerance := time.Duration(float64(expected) * 0.04) // 4% tolerance
	if diff := delay - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected delay of %v (±%v), got %v (%.1f%% difference)", 
			expected, 
			tolerance, 
			delay,
			float64(diff) / float64(expected) * 100,
		)
	}
}

func TestMarsBandwidth(t *testing.T) {
	marsToxic := &toxics.MarsToxic{
		ReferenceTime: time.Date(2018, 7, 27, 0, 0, 0, 0, time.UTC), // At opposition
		Rate:         100, // 100 KB/s
		SpeedOfLight: 299792.458 * 1000, // 1000x normal speed for faster testing
	}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)
	done := make(chan bool)

	go func() {
		marsToxic.Pipe(stub)
		done <- true
	}()

	// Send 50KB of data
	dataSize := 50 * 1024 // 50KB
	
	// At 100 KB/s, 50KB should take exactly 0.5 seconds
	// Expected timing:
	// - Bandwidth delay: 500ms (50KB at 100KB/s)
	// - Mars delay: ~182ms (at opposition, with 1000x speed of light)
	expectedDelay := 500*time.Millisecond + time.Duration(float64(182*time.Second)/1000)

	start := time.Now()
	
	testData := make([]byte, dataSize)
	for i := range testData {
		testData[i] = byte(i % 256) // Fill with recognizable pattern
	}
	
	select {
	case input <- &stream.StreamChunk{
		Data: testData,
	}:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout while sending data")
	}

	// Collect all chunks
	var receivedData []byte
	timeout := time.After(5 * time.Second)
	
	for len(receivedData) < dataSize {
		select {
		case chunk := <-output:
			receivedData = append(receivedData, chunk.Data...)
		case <-timeout:
			t.Fatalf("Timeout while receiving data. Got %d of %d bytes", len(receivedData), dataSize)
		}
	}

	elapsed := time.Since(start)

	// Should take at least 0.5 seconds (50KB at 100KB/s) plus reduced Mars delay
	tolerance := time.Duration(float64(expectedDelay) * 0.04) // 4% tolerance for timing

	if elapsed < expectedDelay-tolerance || elapsed > expectedDelay+tolerance {
		t.Errorf("Expected total delay of %v (±%v), got %v", expectedDelay, tolerance, elapsed)
	}

	if len(receivedData) != dataSize {
		t.Errorf("Expected %d bytes, got %d", dataSize, len(receivedData))
	}

	// Verify data integrity
	for i := range receivedData {
		if receivedData[i] != byte(i%256) {
			t.Errorf("Data corruption at byte %d: expected %d, got %d", i, byte(i%256), receivedData[i])
			break
		}
	}

	close(input)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for toxic to finish")
	}
}

func TestMarsSpeedOfLight(t *testing.T) {
	// Test with 1000x speed of light to reduce delays
	marsToxic := &toxics.MarsToxic{
		ReferenceTime: time.Date(2018, 7, 27, 0, 0, 0, 0, time.UTC), // At opposition
		SpeedOfLight: 299792.458 * 1000, // 1000x normal speed
	}

	delay := marsToxic.Delay()
	expected := time.Duration(float64(182*time.Second) / 1000) // ~182ms (normal 182s / 1000)
	
	tolerance := time.Duration(float64(expected) * 0.04) // 4% tolerance
	if diff := delay - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected delay of %v (±%v), got %v (%.1f%% difference)", 
			expected, 
			tolerance, 
			delay,
			float64(diff) / float64(expected) * 100,
		)
	}
}