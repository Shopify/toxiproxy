package main

import (
	"io"
	"sync"

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
	group  sync.WaitGroup
}

func NewToxicLink(proxy *Proxy, toxics *ToxicCollection) *ToxicLink {
	link := &ToxicLink{stubs: make([]*ToxicStub, MaxToxics), proxy: proxy, toxics: toxics}

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
	for i, toxic := range link.toxics.toxics {
		go link.pipe(toxic, link.stubs[i])
	}
	go func() {
		link.group.Add(1)
		defer link.group.Done()
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

func (link *ToxicLink) pipe(toxic Toxic, stub *ToxicStub) {
	link.group.Add(1)
	toxic.Pipe(stub)
	link.group.Done()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-stub.interrupt:
			case <-done:
				return
			}
		}
	}()
	link.group.Wait()
	close(done)
}

// Replace the toxic at the specified index
func (link *ToxicLink) SetToxic(toxic Toxic, index int) {
	link.stubs[index].Interrupt()
	go link.pipe(toxic, link.stubs[index])
}
