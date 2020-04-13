package toxics

import (
	"github.com/Shopify/toxiproxy/metrics"
	"time"
)

// The NoopToxic passes all data through without any toxic effects, and collects metrics
type NoopToxic struct {
	ProxyName string
	Upstream  string
}

func (t *NoopToxic) Pipe(stub *ToxicStub) {
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}
			metrics.RegisterEvent(metrics.Event{ProxyName: t.ProxyName, Upstream: t.Upstream, Time: time.Now(), EventType: "Message"})
			stub.Output <- c
		}
	}
}

func init() {
	toxic := new(NoopToxic)
	toxic.ProxyName = "fake proxy"
	Register("noop", toxic)
}
