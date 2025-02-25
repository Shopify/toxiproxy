package toxics

import (
	"io"
	"math/rand"

	"github.com/Shopify/toxiproxy/v2/stream"
)

type CorruptToxic struct {
	// probability of bit flips
	Prob float64 `json:"probability"`
}

// reference: https://stackoverflow.com/a/2076028/2708711
func generate_mask(num_bytes int, prob float64, gas int) []byte {
	tol := 0.001
	x := make([]byte, num_bytes)
	rand.Read(x)
	if gas <= 0 {
		return x
	}
	if prob > 0.5+tol {
		y := generate_mask(num_bytes, 2*prob-1, gas-1)
		for i := 0; i < num_bytes; i++ {
			x[i] |= y[i]
		}
		return x
	}
	if prob < 0.5-tol {
		y := generate_mask(num_bytes, 2*prob, gas-1)
		for i := 0; i < num_bytes; i++ {
			x[i] &= y[i]
		}
		return x
	}
	return x
}

func (t *CorruptToxic) corrupt(data []byte) {
	gas := 10
	mask := generate_mask(len(data), t.Prob, gas)
	for i := 0; i < len(data); i++ {
		data[i] ^= mask[i]
	}
}

func (t *CorruptToxic) Pipe(stub *ToxicStub) {
	buf := make([]byte, 32*1024)
	writer := stream.NewChanWriter(stub.Output)
	reader := stream.NewChanReader(stub.Input)
	reader.SetInterrupt(stub.Interrupt)
	for {
		n, err := reader.Read(buf)
		if err == stream.ErrInterrupted {
			t.corrupt(buf[:n])
			writer.Write(buf[:n])
			return
		} else if err == io.EOF {
			stub.Close()
			return
		}
		t.corrupt(buf[:n])
		writer.Write(buf[:n])
	}
}

func init() {
	Register("corrupt", new(CorruptToxic))
}
