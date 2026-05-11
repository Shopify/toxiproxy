package toxics_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
)

func TestLimitTimeToxicContinuesAfterInterrupt(t *testing.T) {
	timeout := int64(1000)
	toxic := &toxics.LimitTimeToxic{Time: timeout}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	// Wait for half the timeout and interrupt
	go func() {
		time.Sleep(time.Duration(timeout/2) * time.Millisecond)
		stub.Interrupt <- struct{}{}
	}()

	start := time.Now()
	toxic.Pipe(stub)
	elapsed1 := time.Since(start)
	if int64(elapsed1/time.Millisecond) >= timeout {
		t.Error("Interrupt did not immediately return from pipe")
	}

	// Without sending anything then pipe should wait for remainder of timeout and close stub
	toxic.Pipe(stub)
	elapsedTotal := time.Since(start)

	if int64(elapsedTotal/time.Millisecond) > int64((float64(timeout) * 1.1)) {
		t.Error("Timeout started again after interrupt")
	}

	if int64(elapsedTotal/time.Millisecond) < timeout {
		t.Error("Did not wait for timeout to elapse")
	}

	if !stub.Closed() {
		t.Error("Did not close pipe after timeout")
	}
}

func TestLimitTimeToxicNilInputShouldClosePipe(t *testing.T) {
	timeout := int64(30000)
	toxic := &toxics.LimitTimeToxic{Time: timeout}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	go func() {
		input <- nil
	}()

	start := time.Now()
	toxic.Pipe(stub)
	elapsed1 := time.Since(start)
	if int64(elapsed1/time.Millisecond) >= timeout {
		t.Error("Did not immediately close pipe")
	}

	if !stub.Closed() {
		t.Error("Did not close pipe")
	}

}

func TestLimitTimeToxicSendsDataThroughBeforeTimeoutReached(t *testing.T) {
	timeout := int64(30000)
	toxic := &toxics.LimitTimeToxic{Time: timeout}

	input := make(chan *stream.StreamChunk)
	output := make(chan *stream.StreamChunk)
	stub := toxics.NewToxicStub(input, output)
	stub.State = toxic.NewState()

	go toxic.Pipe(stub)

	inputBuffer := buffer(100)
	input <- &stream.StreamChunk{Data: inputBuffer}

	sentData := <-output

	if !bytes.Equal(sentData.Data, inputBuffer) {
		t.Error("Data did not get sent through")
	}
}
