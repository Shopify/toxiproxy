package toxiproxy

import (
	"io"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
	"github.com/sirupsen/logrus"
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
	stubs     []*toxics.ToxicStub
	proxy     *Proxy
	toxics    *ToxicCollection
	input     *stream.ChanWriter
	output    *stream.ChanReader
	direction stream.Direction
}

func NewToxicLink(proxy *Proxy, collection *ToxicCollection, direction stream.Direction) *ToxicLink {
	link := &ToxicLink{
		stubs:     make([]*toxics.ToxicStub, len(collection.chain[direction]), cap(collection.chain[direction])),
		proxy:     proxy,
		toxics:    collection,
		direction: direction,
	}

	// Initialize the link with ToxicStubs
	last := make(chan *stream.StreamChunk) // The first toxic is always a noop
	link.input = stream.NewChanWriter(last)
	for i := 0; i < len(link.stubs); i++ {
		var next chan *stream.StreamChunk
		if i+1 < len(link.stubs) {
			next = make(chan *stream.StreamChunk, link.toxics.chain[direction][i+1].BufferSize)
		} else {
			next = make(chan *stream.StreamChunk)
		}

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
				"name":  link.proxy.Name,
				"bytes": bytes,
				"err":   err,
			}).Warn("Source terminated")
		}
		link.input.Close()
	}()
	for i, toxic := range link.toxics.chain[link.direction] {
		if stateful, ok := toxic.Toxic.(toxics.StatefulToxic); ok {
			link.stubs[i].State = stateful.NewState()
		}

		go link.stubs[i].Run(toxic)
	}
	go func() {
		bytes, err := io.Copy(dest, link.output)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":  link.proxy.Name,
				"bytes": bytes,
				"err":   err,
			}).Warn("Destination terminated")
		}
		dest.Close()
		link.toxics.RemoveLink(name)
		link.proxy.RemoveConnection(name)
	}()
}

// Add a toxic to the end of the chain.
func (link *ToxicLink) AddToxic(toxic *toxics.ToxicWrapper) {
	i := len(link.stubs)

	newin := make(chan *stream.StreamChunk, toxic.BufferSize)
	link.stubs = append(link.stubs, toxics.NewToxicStub(newin, link.stubs[i-1].Output))

	// Interrupt the last toxic so that we don't have a race when moving channels
	if link.stubs[i-1].InterruptToxic() {
		link.stubs[i-1].Output = newin

		if stateful, ok := toxic.Toxic.(toxics.StatefulToxic); ok {
			link.stubs[i].State = stateful.NewState()
		}

		go link.stubs[i].Run(toxic)
		go link.stubs[i-1].Run(link.toxics.chain[link.direction][i-1])
	} else {
		// This link is already closed, make sure the new toxic matches
		link.stubs[i].Output = newin // The real output is already closed, close this instead
		link.stubs[i].Close()
	}
}

// Update an existing toxic in the chain.
func (link *ToxicLink) UpdateToxic(toxic *toxics.ToxicWrapper) {
	if link.stubs[toxic.Index].InterruptToxic() {
		go link.stubs[toxic.Index].Run(toxic)
	}
}

// Remove an existing toxic from the chain.
func (link *ToxicLink) RemoveToxic(toxic *toxics.ToxicWrapper) {
	i := toxic.Index

	if link.stubs[i].InterruptToxic() {
		cleanup, ok := toxic.Toxic.(toxics.CleanupToxic)
		if ok {
			cleanup.Cleanup(link.stubs[i])
			// Cleanup could have closed the stub.
			if link.stubs[i].Closed() {
				return
			}
		}

		stop := make(chan bool)
		// Interrupt the previous toxic to update its output
		go func() {
			stop <- link.stubs[i-1].InterruptToxic()
		}()

		// Unblock the previous toxic if it is trying to flush
		// If the previous toxic is closed, continue flusing until we reach the end.
		interrupted := false
		stopped := false
		for !interrupted {
			select {
			case interrupted = <-stop:
				stopped = true
			case tmp := <-link.stubs[i].Input:
				if tmp == nil {
					link.stubs[i].Close()
					if !stopped {
						<-stop
					}
					return
				}
				link.stubs[i].Output <- tmp
			}
		}

		// Empty the toxic's buffer if necessary
		for len(link.stubs[i].Input) > 0 {
			tmp := <-link.stubs[i].Input
			if tmp == nil {
				link.stubs[i].Close()
				return
			}
			link.stubs[i].Output <- tmp
		}

		link.stubs[i-1].Output = link.stubs[i].Output
		link.stubs = append(link.stubs[:i], link.stubs[i+1:]...)

		go link.stubs[i-1].Run(link.toxics.chain[link.direction][i-1])
	}
}
