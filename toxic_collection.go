package main

type ToxicCollection struct {
	noop              *NoopToxic
	LatencyUpstream   *LatencyToxic `json:"latency_upstream"`
	LatencyDownstream *LatencyToxic `json:"latency_downstream"`
}

// Constants used to define which index in link.upToxics / link.downToxics
// each type of toxic uses.
const (
	LatencyIndex = iota
	MaxToxics
)

func NewToxicCollection() *ToxicCollection {
	return &ToxicCollection{
		noop:              new(NoopToxic),
		LatencyUpstream:   new(LatencyToxic),
		LatencyDownstream: new(LatencyToxic),
	}
}
