package toxiproxy

import (
	"context"
	"encoding/binary"
	"flag"
	"io"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/testhelper"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

func TestToxicsAreLoaded(t *testing.T) {
	if toxics.Count() < 1 {
		t.Fatal("No toxics loaded!")
	}
}

func TestStubInitializaation(t *testing.T) {
	collection := NewToxicCollection(nil)
	link := NewToxicLink(nil, collection, stream.Downstream, zerolog.Nop())
	if len(link.stubs) != 1 {
		t.Fatalf("Link created with wrong number of stubs: %d != 1", len(link.stubs))
	}

	if cap(link.stubs) != toxics.Count()+1 {
		t.Fatalf("Link created with wrong capacity: %d != %d", cap(link.stubs), toxics.Count()+1)
	}

	if cap(link.stubs[0].Input) != 0 {
		t.Fatalf("Noop buffer was not initialized as 0: %d", cap(link.stubs[0].Input))
	}

	if cap(link.stubs[0].Output) != 0 {
		t.Fatalf("Link output buffer was not initialized as 0: %d", cap(link.stubs[0].Output))
	}
}

func TestStubInitializaationWithToxics(t *testing.T) {
	collection := NewToxicCollection(nil)
	collection.chainAddToxic(&toxics.ToxicWrapper{
		Toxic:      new(toxics.LatencyToxic),
		Type:       "latency",
		Direction:  stream.Downstream,
		BufferSize: 1024,
		Toxicity:   1,
	})
	collection.chainAddToxic(&toxics.ToxicWrapper{
		Toxic:     new(toxics.BandwidthToxic),
		Type:      "bandwidth",
		Direction: stream.Downstream,
		Toxicity:  1,
	})
	link := NewToxicLink(nil, collection, stream.Downstream, zerolog.Nop())

	if len(link.stubs) != 3 {
		t.Fatalf("Link created with wrong number of stubs: %d != 3", len(link.stubs))
	}

	if cap(link.stubs) != toxics.Count()+1 {
		t.Fatalf("Link created with wrong capacity: %d != %d", cap(link.stubs), toxics.Count()+1)
	}

	if cap(link.stubs[len(link.stubs)-1].Output) != 0 {
		t.Fatalf("Link output buffer was not initialized as 0: %d", cap(link.stubs[0].Output))
	}

	for i, toxic := range collection.chain[stream.Downstream] {
		if cap(link.stubs[i].Input) != toxic.BufferSize {
			t.Fatalf(
				"%s buffer was not initialized as %d: %d",
				toxic.Type,
				toxic.BufferSize,
				cap(link.stubs[i].Input),
			)
		}
	}
}

func TestAddRemoveStubs(t *testing.T) {
	ctx := context.Background()
	collection := NewToxicCollection(nil)
	dummyProxy := NewProxy(NewServer(nil, zerolog.Nop(), time.Now().UnixNano()),
		"DummyProxy",
		"localhost:0",
		"upstream")
	link := NewToxicLink(dummyProxy, collection, stream.Downstream, zerolog.Nop())
	go link.stubs[0].Run(collection.chain[stream.Downstream][0], link.proxy.apiServer.seed)
	collection.links["test"] = link

	// Add stubs
	collection.chainAddToxic(&toxics.ToxicWrapper{
		Toxic:      new(toxics.LatencyToxic),
		Type:       "latency",
		Direction:  stream.Downstream,
		BufferSize: 1024,
		Toxicity:   1,
	})
	toxic := &toxics.ToxicWrapper{
		Toxic:      new(toxics.BandwidthToxic),
		Type:       "bandwidth",
		Direction:  stream.Downstream,
		BufferSize: 2048,
		Toxicity:   1,
	}
	collection.chainAddToxic(toxic)
	if cap(link.stubs[len(link.stubs)-1].Output) != 0 {
		t.Fatalf("Link output buffer was not initialized as 0: %d", cap(link.stubs[0].Output))
	}
	for i, toxic := range collection.chain[stream.Downstream] {
		if cap(link.stubs[i].Input) != toxic.BufferSize {
			t.Fatalf(
				"%s buffer was not initialized as %d: %d",
				toxic.Type,
				toxic.BufferSize,
				cap(link.stubs[i].Input),
			)
		}
	}

	// Remove stubs
	collection.chainRemoveToxic(ctx, toxic)
	if cap(link.stubs[len(link.stubs)-1].Output) != 0 {
		t.Fatalf("Link output buffer was not initialized as 0: %d", cap(link.stubs[0].Output))
	}
	for i, toxic := range collection.chain[stream.Downstream] {
		if cap(link.stubs[i].Input) != toxic.BufferSize {
			t.Fatalf(
				"%s buffer was not initialized as %d: %d",
				toxic.Type,
				toxic.BufferSize,
				cap(link.stubs[i].Input),
			)
		}
	}
}

func TestNoDataDropped(t *testing.T) {
	ctx := context.Background()
	collection := NewToxicCollection(nil)
	dummyProxy := NewProxy(NewServer(nil, zerolog.Nop(), time.Now().UnixNano()),
		"DummyProxy",
		"localhost:0",
		"upstream")
	link := NewToxicLink(dummyProxy, collection, stream.Downstream, zerolog.Nop())
	go link.stubs[0].Run(collection.chain[stream.Downstream][0], link.proxy.apiServer.seed)
	collection.links["test"] = link

	toxic := &toxics.ToxicWrapper{
		Toxic: &toxics.LatencyToxic{
			Latency: 1000,
		},
		Type:       "latency",
		Direction:  stream.Downstream,
		BufferSize: 1024,
		Toxicity:   1,
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		for i := uint16(0); i < 65535; i++ {
			buf := make([]byte, 2)
			binary.BigEndian.PutUint16(buf, i)
			link.input.Write(buf)
		}
		link.input.Close()
	}()
	go func(ctx context.Context) {
		for {
			select {
			case <-done:
				return
			default:
				collection.chainAddToxic(toxic)
				collection.chainRemoveToxic(ctx, toxic)
			}
		}
	}(ctx)

	buf := make([]byte, 2)
	for i := uint16(0); i < 65535; i++ {
		n, err := link.output.Read(buf)
		if n != 2 || err != nil {
			t.Fatalf("Read failed: %d %v", n, err)
		} else {
			val := binary.BigEndian.Uint16(buf)
			if val != i {
				t.Fatalf("Read incorrect bytes: %v != %d", val, i)
			}
		}
	}
	n, err := link.output.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatalf("Expected EOF: %d %v", n, err)
	}
}

func TestToxicity(t *testing.T) {
	collection := NewToxicCollection(nil)
	dummyProxy := NewProxy(NewServer(nil, zerolog.Nop(), time.Now().UnixNano()),
		"DummyProxy",
		"localhost:0",
		"upstream")
	link := NewToxicLink(dummyProxy, collection, stream.Downstream, zerolog.Nop())
	go link.stubs[0].Run(collection.chain[stream.Downstream][0], link.proxy.apiServer.seed)
	collection.links["test"] = link

	toxic := &toxics.ToxicWrapper{
		Toxic:     new(toxics.TimeoutToxic),
		Name:      "timeout1",
		Type:      "timeout",
		Direction: stream.Downstream,
		Toxicity:  0,
	}
	collection.chainAddToxic(toxic)

	// Toxic should be a Noop because of toxicity
	n, err := link.input.Write([]byte{42})
	if n != 1 || err != nil {
		t.Fatalf("Write failed: %d %v", n, err)
	}
	buf := make([]byte, 2)
	n, err = link.output.Read(buf)
	if n != 1 || err != nil {
		t.Fatalf("Read failed: %d %v", n, err)
	} else if buf[0] != 42 {
		t.Fatalf("Read wrong byte: %x", buf[0])
	}

	toxic.Toxicity = 1
	toxic.Toxic.(*toxics.TimeoutToxic).Timeout = 100
	collection.chainUpdateToxic(toxic)

	err = testhelper.TimeoutAfter(150*time.Millisecond, func() {
		n, err = link.input.Write([]byte{42})
		if n != 1 || err != nil {
			t.Fatalf("Write failed: %d %v", n, err)
		}
		n, err = link.output.Read(buf)
		if n != 0 || err != io.EOF {
			t.Fatalf("Read did not get EOF: %d %v", n, err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStateCreated(t *testing.T) {
	collection := NewToxicCollection(nil)
	log := zerolog.Nop()
	if flag.Lookup("test.v").DefValue == "true" {
		log = zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	}
	dummyServer := NewServer(nil, log, time.Now().UnixNano())
	dummyProxy := NewProxy(dummyServer, "DummyProxy", "localhost:0", "upstream")
	link := NewToxicLink(dummyProxy, collection, stream.Downstream, log)
	go link.stubs[0].Run(collection.chain[stream.Downstream][0], link.proxy.apiServer.seed)
	collection.links["test"] = link

	collection.chainAddToxic(&toxics.ToxicWrapper{
		Toxic:     new(toxics.LimitDataToxic),
		Type:      "limit_data",
		Direction: stream.Downstream,
		Toxicity:  1,
	})
	if link.stubs[len(link.stubs)-1].State == nil {
		t.Fatalf("New toxic did not have state object created.")
	}
}

func TestRemoveToxicWithBrokenConnection(t *testing.T) {
	ctx := context.Background()

	log := zerolog.Nop()
	if flag.Lookup("test.v").DefValue == "true" {
		log = zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	}
	ctx = log.WithContext(ctx)
	collection := NewToxicCollection(nil)
	dummyServer := NewServer(nil, log, time.Now().UnixNano())
	dummyProxy := NewProxy(dummyServer, "DummyProxy", "localhost:0", "upstream")
	link := NewToxicLink(dummyProxy, collection, stream.Downstream, log)
	go link.stubs[0].Run(collection.chain[stream.Downstream][0], link.proxy.apiServer.seed)
	collection.links["test"] = link

	toxics := [2]*toxics.ToxicWrapper{
		{
			Toxic: &toxics.BandwidthToxic{
				Rate: 0,
			},
			Type:      "bandwidth",
			Direction: stream.Downstream,
			Toxicity:  1,
		},
		{
			Toxic: &toxics.BandwidthToxic{
				Rate: 0,
			},
			Type:      "bandwidth",
			Direction: stream.Upstream,
			Toxicity:  1,
		},
	}

	collection.chainAddToxic(toxics[0])
	collection.chainAddToxic(toxics[1])

	done := make(chan struct{})
	defer close(done)

	var data uint16 = 42
	go func(log zerolog.Logger) {
		for {
			select {
			case <-done:
				link.input.Close()
				return
			case <-time.After(10 * time.Second):
				log.Print("Finish load")
				return
			default:
				buf := make([]byte, 2)
				binary.BigEndian.PutUint16(buf, data)
				link.input.Write(buf)
			}
		}
	}(log)

	collection.chainRemoveToxic(ctx, toxics[0])
	collection.chainRemoveToxic(ctx, toxics[1])
}

func TestStableToxicityWithSeed(t *testing.T) {
	collection := NewToxicCollection(nil)
	//for a seed == 1 the random number generated is 0.604(...)
	dummyProxy := NewProxy(NewServer(nil, zerolog.Nop(), 1),
		"DummyProxy",
		"localhost:0",
		"upstream")
	link := NewToxicLink(dummyProxy, collection, stream.Downstream, zerolog.Nop())
	go link.stubs[0].Run(collection.chain[stream.Downstream][0], link.proxy.apiServer.seed)
	collection.links["test"] = link

	toxic := &toxics.ToxicWrapper{
		Toxic:     new(toxics.TimeoutToxic),
		Name:      "timeout1",
		Type:      "timeout",
		Direction: stream.Downstream,
		Toxicity:  0.603,
	}
	collection.chainAddToxic(toxic)

	// Toxic should be a Noop because of toxicity
	n, err := link.input.Write([]byte{42})
	if n != 1 || err != nil {
		t.Fatalf("Write failed: %d %v", n, err)
	}
	buf := make([]byte, 2)
	n, err = link.output.Read(buf)
	if n != 1 || err != nil {
		t.Fatalf("Read failed: %d %v", n, err)
	} else if buf[0] != 42 {
		t.Fatalf("Read wrong byte: %x", buf[0])
	}

	toxic.Toxicity = 0.605
	toxic.Toxic.(*toxics.TimeoutToxic).Timeout = 100
	collection.chainUpdateToxic(toxic)

	err = testhelper.TimeoutAfter(150*time.Millisecond, func() {
		n, err = link.input.Write([]byte{42})
		if n != 1 || err != nil {
			t.Fatalf("Write failed: %d %v", n, err)
		}
		n, err = link.output.Read(buf)
		if n != 0 || err != io.EOF {
			t.Fatalf("Read did not get EOF: %d %v", n, err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
