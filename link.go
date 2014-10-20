package main

import "io"

// ToxicLinks are single direction pipelines that connects an input and output via
// a chain of toxics. There is a fixed number of toxics in the chain, such that a
// toxic always maps to the same toxic stub. Toxics are replaced with noops when
// disabled.
//
//         NoopToxic LatencyToxic  NoopToxic
//             v           v           v
// Input > ToxicStub > ToxicStub > ToxicStub > Output
//
type ToxicLink []*ToxicStub

func NewToxicLink(proxy *Proxy, input io.Reader, output io.WriteCloser) ToxicLink {
	link := ToxicLink(make([]*ToxicStub, MaxToxics))

	// Initialize the link with ToxicStubs
	var last io.Reader = input
	for i := 0; i < MaxToxics; i++ {
		if i == MaxToxics-1 {
			link[i] = NewToxicStub(proxy, last, output)
		} else {
			r, w := io.Pipe()
			link[i] = NewToxicStub(proxy, last, w)
			last = r
		}
	}
	return link
}

// Start the link with the specified toxics
func (link ToxicLink) Start(toxics []Toxic) {
	for i, toxic := range toxics {
		go toxic.Pipe(link[i])
	}
}

// Replace the toxic at the specified index
func (link ToxicLink) SetToxic(toxic Toxic, index int) {
	link[index].Interrupt()
	go toxic.Pipe(link[index])
}
