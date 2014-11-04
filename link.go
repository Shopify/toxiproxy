package main

import (
	"io"

	"github.com/Sirupsen/logrus"
)

// ToxicLinks are single direction pipelines that connects an input and output via
// a chain of toxics. There is a fixed number of toxics in the chain, such that a
// toxic always maps to the same toxic stub. Toxics are replaced with noops when
// disabled.
//
//         NoopToxic LatencyToxic  NoopToxic
//             v           v           v
// Input > ToxicStub > ToxicStub > ToxicStub > Output
//
type ToxicLink struct {
	stubs  []*ToxicStub
	proxy  *Proxy
	input  ChanWriter
	output ChanReader
}

func NewToxicLink(proxy *Proxy) *ToxicLink {
	link := &ToxicLink{stubs: make([]*ToxicStub, MaxToxics), proxy: proxy}

	// Initialize the link with ToxicStubs
	last := make(chan []byte)
	link.input = NewChanWriter(last)
	for i := 0; i < MaxToxics; i++ {
		next := make(chan []byte)
		link.stubs[i] = NewToxicStub(proxy, last, next)
		last = next
	}
	link.output = NewChanReader(last)
	return link
}

// Start the link with the specified toxics
func (link *ToxicLink) Start(toxics []Toxic, input io.Reader, output io.WriteCloser) {
	go func() {
		bytes, err := PacketizeCopy(link.input, input)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":     link.proxy.Name,
				"upstream": link.proxy.Upstream,
				"bytes":    bytes,
				"err":      err,
			}).Warn("Source terminated")
		}
		link.input.Close()
	}()
	for i, toxic := range toxics {
		go toxic.Pipe(link.stubs[i])
	}
	go func() {
		bytes, err := io.Copy(output, link.output)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":     link.proxy.Name,
				"upstream": link.proxy.Upstream,
				"bytes":    bytes,
				"err":      err,
			}).Warn("Destination terminated")
		}
		output.Close()
	}()
}

// Replace the toxic at the specified index
func (link *ToxicLink) SetToxic(toxic Toxic, index int) {
	link.stubs[index].Interrupt()
	go toxic.Pipe(link.stubs[index])
}
