package toxics_test

import (
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
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
	
	tolerance := time.Duration(float64(expected) * 0.03) // 3% tolerance
	if diff := delay - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected delay of %v (±%v), got %v (%.1f%% difference)", 
			expected, 
			tolerance, 
			delay,
			float64(diff) / float64(expected) * 100,
		)
	}
}