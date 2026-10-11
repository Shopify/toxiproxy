// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x
package toxiproxy

type Attributes map[string]interface{}

type Toxic struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Stream     string     `json:"stream,omitempty"`
	Toxicity   float32    `json:"toxicity"`
	Attributes Attributes `json:"attributes"`
}

type Toxics []Toxic

type ToxicOptions struct {
	ProxyName,
	ToxicName,
	ToxicType,
	Stream string
	Toxicity   float32
	Attributes Attributes
}
