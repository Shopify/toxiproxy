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
	toxics *ToxicCollection
	input  *ChanWriter
	output *ChanReader
}

func NewToxicLink(proxy *Proxy, toxics *ToxicCollection) *ToxicLink {
	link := &ToxicLink{
		stubs:  make([]*ToxicStub, len(toxics.chain)),
		proxy:  proxy,
		toxics: toxics,
	}

	// Initialize the link with ToxicStubs
	last := make(chan *StreamChunk, 1024)
	link.input = NewChanWriter(last)
	for i := 0; i < len(link.stubs); i++ {
		next := make(chan *StreamChunk, 1024)
		link.stubs[i] = NewToxicStub(last, next)
		last = next
	}
	link.output = NewChanReader(last)
	return link
}

// Start the link with the specified toxics
func (link *ToxicLink) Start(name string, source io.Reader, dest io.WriteCloser) {
	go func() {
		bytes, err := io.Copy(link.input, source)
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
	for i, toxic := range link.toxics.chain {
		go link.stubs[i].Run(toxic)
	}
	go func() {
		bytes, err := io.Copy(dest, link.output)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":     link.proxy.Name,
				"upstream": link.proxy.Upstream,
				"bytes":    bytes,
				"err":      err,
			}).Warn("Destination terminated")
		}
		dest.Close()
		link.toxics.RemoveLink(name)
		link.proxy.RemoveConnection(name)
	}()
}

// Replace the toxic at the specified index
func (link *ToxicLink) SetToxic(toxic Toxic, index int) {
	if link.stubs[index].Interrupt() {
		go link.stubs[index].Run(toxic)
	}
}
