package toxiproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/rs/zerolog"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

// ToxicLinks are single direction pipelines that connects an input and output via
// a chain of toxics. The chain always starts with a NoopToxic, and toxics are added
// and removed as they are enabled/disabled. New toxics are always added to the end
// of the chain.
//
// |         NoopToxic  LatencyToxic
// |             v           v
// | Input > ToxicStub > ToxicStub > Output.
type ToxicLink struct {
	stubs     []*toxics.ToxicStub
	proxy     *Proxy
	toxics    *ToxicCollection
	input     *stream.ChanWriter
	output    *stream.ChanReader
	direction stream.Direction
	Logger    *zerolog.Logger
}

func NewToxicLink(
	proxy *Proxy,
	collection *ToxicCollection,
	direction stream.Direction,
	logger zerolog.Logger,
) *ToxicLink {
	link := &ToxicLink{
		stubs: make(
			[]*toxics.ToxicStub,
			len(collection.chain[direction]),
			cap(collection.chain[direction]),
		),
		proxy:     proxy,
		toxics:    collection,
		direction: direction,
		Logger:    &logger,
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

// Start the link with the specified toxics.
func (link *ToxicLink) Start(
	server *ApiServer,
	name string,
	source io.Reader,
	dest io.WriteCloser,
) {
	logger := link.Logger
	logger.
		Debug().
		Str("direction", link.Direction()).
		Msg("Setup connection")

	labels := []string{
		link.Direction(),
		link.proxy.Name,
		link.proxy.Listen,
		link.proxy.Upstream}

	go link.read(labels, server, source)

	for i, toxic := range link.toxics.chain[link.direction] {
		if stateful, ok := toxic.Toxic.(toxics.StatefulToxic); ok {
			link.stubs[i].State = stateful.NewState()
		}

		if _, ok := toxic.Toxic.(*toxics.ResetToxic); ok {
			if err := source.(*net.TCPConn).SetLinger(0); err != nil {
				logger.Err(err).
					Str("toxic", toxic.Type).
					Msg("source: Unable to setLinger(ms)")
			}

			if err := dest.(*net.TCPConn).SetLinger(0); err != nil {
				logger.Err(err).
					Str("toxic", toxic.Type).
					Msg("dest: Unable to setLinger(ms)")
			}
		}

		go link.stubs[i].Run(toxic, server.seed)
	}

	go link.write(labels, name, server, dest)
}

// read copies bytes from a source to the link's input channel.
func (link *ToxicLink) read(
	metricLabels []string,
	server *ApiServer,
	source io.Reader,
) {
	logger := link.Logger
	bytes, err := io.Copy(link.input, source)
	if err != nil {
		logger.Warn().
			Int64("bytes", bytes).
			Err(err).
			Msg("Source terminated")
	}
	if server.Metrics.proxyMetricsEnabled() {
		server.Metrics.ProxyMetrics.ReceivedBytesTotal.
			WithLabelValues(metricLabels...).Add(float64(bytes))
	}
	link.input.Close()
}

// write copies bytes from the link's output channel to a destination.
func (link *ToxicLink) write(
	metricLabels []string,
	name string,
	server *ApiServer, // TODO: Replace with AppConfig for Metrics and Logger
	dest io.WriteCloser,
) {
	logger := link.Logger.
		With().
		Str("component", "ToxicLink").
		Str("method", "write").
		Str("link", name).
		Str("proxy", link.proxy.Name).
		Str("link_addr", fmt.Sprintf("%p", link)).
		Logger()

	bytes, err := io.Copy(dest, link.output)
	if err != nil {
		logger.Warn().
			Int64("bytes", bytes).
			Err(err).
			Msg("Could not write to destination")
	} else if server.Metrics.proxyMetricsEnabled() {
		server.Metrics.ProxyMetrics.SentBytesTotal.
			WithLabelValues(metricLabels...).Add(float64(bytes))
	}

	dest.Close()
	logger.Trace().Msgf("Remove link %s from ToxicCollection", name)
	link.toxics.RemoveLink(name)
	logger.Trace().Msgf("RemoveConnection %s from Proxy %s", name, link.proxy.Name)
	link.proxy.RemoveConnection(name)
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

		go link.stubs[i].Run(toxic, link.proxy.apiServer.seed)
		go link.stubs[i-1].Run(link.toxics.chain[link.direction][i-1], link.proxy.apiServer.seed)
	} else {
		// This link is already closed, make sure the new toxic matches
		link.stubs[i].Output = newin // The real output is already closed, close this instead
		link.stubs[i].Close()
	}
}

// Update an existing toxic in the chain.
func (link *ToxicLink) UpdateToxic(toxic *toxics.ToxicWrapper) {
	if link.stubs[toxic.Index].InterruptToxic() {
		go link.stubs[toxic.Index].Run(toxic, link.proxy.apiServer.seed)
	}
}

// Remove an existing toxic from the chain.
func (link *ToxicLink) RemoveToxic(ctx context.Context, toxic *toxics.ToxicWrapper) {
	toxic_index := toxic.Index
	log := zerolog.Ctx(ctx).
		With().
		Str("component", "ToxicLink").
		Str("method", "RemoveToxic").
		Str("toxic", toxic.Name).
		Str("toxic_type", toxic.Type).
		Int("toxic_index", toxic.Index).
		Str("link_addr", fmt.Sprintf("%p", link)).
		Str("toxic_stub_addr", fmt.Sprintf("%p", link.stubs[toxic_index])).
		Str("prev_toxic_stub_addr", fmt.Sprintf("%p", link.stubs[toxic_index-1])).
		Logger()

	if link.stubs[toxic_index].InterruptToxic() {
		cleanup, ok := toxic.Toxic.(toxics.CleanupToxic)
		if ok {
			cleanup.Cleanup(link.stubs[toxic_index])
			// Cleanup could have closed the stub.
			if link.stubs[toxic_index].Closed() {
				log.Trace().Msg("Cleanup closed toxic and removed toxic")
				// TODO: Check if cleanup happen would link.stubs recalculated?
				return
			}
		}

		log.Trace().Msg("Interrupting the previous toxic to update its output")
		stop := make(chan bool)
		go func(stub *toxics.ToxicStub, stop chan bool) {
			stop <- stub.InterruptToxic()
		}(link.stubs[toxic_index-1], stop)

		// Unblock the previous toxic if it is trying to flush
		// If the previous toxic is closed, continue flusing until we reach the end.
		interrupted := false
		stopped := false
		for !interrupted {
			select {
			case interrupted = <-stop:
				stopped = true
			case tmp := <-link.stubs[toxic_index].Input:
				if tmp == nil {
					link.stubs[toxic_index].Close()
					if !stopped {
						<-stop
					}
					return // TODO: There are some steps after this to clean buffer
				}

				err := link.stubs[toxic_index].WriteOutput(tmp, 5*time.Second)
				if err != nil {
					log.Err(err).
						Msg("Could not write last packets after interrupt to Output")
				}
			}
		}

		// Empty the toxic's buffer if necessary
		for len(link.stubs[toxic_index].Input) > 0 {
			tmp := <-link.stubs[toxic_index].Input
			if tmp == nil {
				link.stubs[toxic_index].Close()
				return
			}
			err := link.stubs[toxic_index].WriteOutput(tmp, 5*time.Second)
			if err != nil {
				log.Err(err).
					Msg("Could not write last packets after interrupt to Output")
			}
		}

		link.stubs[toxic_index-1].Output = link.stubs[toxic_index].Output
		link.stubs = append(link.stubs[:toxic_index], link.stubs[toxic_index+1:]...)

		go link.stubs[toxic_index-1].Run(link.toxics.chain[link.direction][toxic_index-1],
			link.proxy.apiServer.seed)
	}
}

// Direction returns the direction of the link (upstream or downstream).
func (link *ToxicLink) Direction() string {
	return link.direction.String()
}
