// Ported from https://github.com/xthexder/toxic-example/blob/master/noop.go

package main

import (
	"os"
	"log"
	"fmt"
	"io"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/Shopify/toxiproxy/v2"
	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/Shopify/toxiproxy/v2/stream"
)

// DebugToxic prints bytes processed through pipe.
type DebugToxic struct{}

func (t *DebugToxic) PrintHex(data []byte) {
	for i := 0; i < len(data); {
		for j := 0; j < 4; j +=1 {
			x := i + 8
			if x >= len(data) {
				x = len(data) - 1
				fmt.Printf("% x\n", data[i:x])
				return
			}
			fmt.Printf("% x\t\t", data[i:x])
			i = x
		}
		fmt.Println()
	}
}

func (t *DebugToxic) Pipe(stub *toxics.ToxicStub) {
	buf := make([]byte, 32*1024)
	writer := stream.NewChanWriter(stub.Output)
	reader := stream.NewChanReader(stub.Input)
	reader.SetInterrupt(stub.Interrupt)
	for {
			n, err := reader.Read(buf)
			log.Printf("-- [DebugToxic] Processed %d bytes\n", n)
			if err == stream.ErrInterrupted {
					writer.Write(buf[:n])
					return
			} else if err == io.EOF {
					stub.Close()
					return
			}
			t.PrintHex(buf[:n])
			writer.Write(buf[:n])
	}

}

func main() {
	toxics.Register("debug", new(DebugToxic))

	logger := zerolog.New(os.Stderr).With().Caller().Timestamp().Logger()
	metrics := toxiproxy.NewMetricsContainer(prometheus.NewRegistry())
	server := toxiproxy.NewServer(metrics, logger)
	server.Listen("0.0.0.0:8484")
}
