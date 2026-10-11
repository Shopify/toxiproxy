package stream_test

import (
	"testing"

	"github.com/Shopify/toxiproxy/v2/stream"
)

func TestDirection_String(t *testing.T) {
	testCases := []struct {
		name      string
		direction stream.Direction
		expected  string
	}{
		{"Downstream to string", stream.Downstream, "downstream"},
		{"Upstream to string", stream.Upstream, "upstream"},
		{"NumDirections to string", stream.NumDirections, "num_directions"},
		{"Upstream via number direction to string", stream.Direction(0), "upstream"},
		{"Downstream via number direction to string", stream.Direction(1), "downstream"},
		{"High number direction to string", stream.Direction(5), "num_directions"},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := tc.direction.String()
			if actual != tc.expected {
				t.Errorf("got \"%s\"; expected \"%s\"", actual, tc.expected)
			}
		})
	}
}

func TestParseDirection(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected stream.Direction
		err      error
	}{
		{"parse empty", "", stream.NumDirections, stream.ErrInvalidDirectionParameter},
		{"parse upstream", "upstream", stream.Upstream, nil},
		{"parse downstream", "downstream", stream.Downstream, nil},
		{"parse unknown", "unknown", stream.NumDirections, stream.ErrInvalidDirectionParameter},
		{"parse number", "-123", stream.NumDirections, stream.ErrInvalidDirectionParameter},
		{"parse upper case", "DOWNSTREAM", stream.Downstream, nil},
		{"parse camel case", "UpStream", stream.Upstream, nil},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual, err := stream.ParseDirection(tc.input)
			if actual != tc.expected {
				t.Errorf("got \"%s\"; expected \"%s\"", actual, tc.expected)
			}

			if err != tc.err {
				t.Errorf("got \"%s\"; expected \"%s\"", err, tc.err)
			}
		})
	}
}
