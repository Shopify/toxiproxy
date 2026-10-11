package stream

import (
	"errors"
	"strings"
)

type Direction uint8

var ErrInvalidDirectionParameter error = errors.New("stream: invalid direction")

const (
	Upstream Direction = iota
	Downstream
	NumDirections
)

func (d Direction) String() string {
	if d >= NumDirections {
		return "num_directions"
	}
	return [...]string{"upstream", "downstream"}[d]
}

func ParseDirection(value string) (Direction, error) {
	switch strings.ToLower(value) {
	case "downstream":
		return Downstream, nil
	case "upstream":
		return Upstream, nil
	}

	return NumDirections, ErrInvalidDirectionParameter
}
