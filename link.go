package toxiproxy

import (
	"io"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
	"github.com/Sirupsen/logrus"
)

// ToxicLinks are single direction pipelines that connects an input and output via
// a chain of toxics. The chain always starts with a NoopToxic, and toxics are added
// and removed as they are enabled/disabled. New toxics are always added to the end
// of the chain.
//
//         NoopToxic  LatencyToxic
//             v           v
// Input > ToxicStub > ToxicStub > Output
//
type ToxicLink struct {
	stubs  []*toxics.ToxicStub
	proxy  *Proxy
	toxics *ToxicCollection
	input  *stream.ChanWriter
	output *stream.ChanReader
}

func NewToxicLink(proxy *Proxy, collection *ToxicCollection) *ToxicLink {
	link := &ToxicLink{
		stubs:  make([]*toxics.ToxicStub, len(collection.chain), cap(collection.chain)),
		proxy:  proxy,
		toxics: collection,
	}

	// Initialize the link with ToxicStubs
	last := make(chan *stream.StreamChunk, 1024)
	link.input = stream.NewChanWriter(last)
	for i := 0; i < len(link.stubs); i++ {
		next := make(chan *stream.StreamChunk, 1024)
		link.stubs[i] = toxics.NewToxicStub(last, next)
		last = next
	}
	link.output = stream.NewChanReader(last)
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

// Add a toxic to the end of the chain.
func (link *ToxicLink) AddToxic(toxic *toxics.ToxicWrapper) {
	i := toxic.Index

	// Interrupt the last toxic so that we don't have a race when moving channels
	if link.stubs[i-1].InterruptToxic() {
		newin := make(chan *stream.StreamChunk)
		link.stubs = append(link.stubs, toxics.NewToxicStub(newin, link.stubs[i-1].Output))
		link.stubs[i-1].Output = newin

		go link.stubs[i].Run(toxic)
		go link.stubs[i-1].Run(link.toxics.chain[i-1])
	}
}

// Replace an existing toxic in the chain.
func (link *ToxicLink) UpdateToxic(toxic *toxics.ToxicWrapper) {
	if link.stubs[toxic.Index].InterruptToxic() {
		go link.stubs[toxic.Index].Run(toxic)
	}
}

// Remove an existing toxic from the chain.
func (link *ToxicLink) RemoveToxic(toxic *toxics.ToxicWrapper) {
	i := toxic.Index

	// Interrupt the last toxic so that the target's buffer is empty
	if link.stubs[i-1].InterruptToxic() && link.stubs[i].InterruptToxic() {
		link.stubs[i-1].Output = link.stubs[i].Output
		link.stubs = append(link.stubs[:i], link.stubs[i+1:]...)

		go link.stubs[i-1].Run(link.toxics.chain[i-1])
	}

}
