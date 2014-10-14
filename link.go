package main

import "io"

// ProxyLink is a single direction pipeline that connects an input and output via
// a chain of toxics. There is a fixed number of toxics in the chain, and they are
// so any disabled toxics are replaced with NoopToxics.
//
//         NoopToxic LatencyToxic  NoopToxic
//             v           v           v
// Input > ToxicStub > ToxicStub > ToxicStub > Output
//
type ProxyLink []*ToxicStub

func NewProxyLink(proxy *Proxy, input io.Reader, output io.WriteCloser) ProxyLink {
	link := ProxyLink(make([]*ToxicStub, MaxToxics))

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
func (link ProxyLink) Start(toxics []Toxic) {
	for i, toxic := range toxics {
		go toxic.Pipe(link[i])
	}
}

// Replace the toxic at the specified index
func (link ProxyLink) SetToxic(toxic Toxic, index int) {
	link[index].Interrupt()
	go toxic.Pipe(link[index])
}
