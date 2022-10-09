package toxics

import (
	"math/rand"
)

type CorruptToxic struct {
	// probability of bit flips
	Prob float64 `json:"probability"`
}

// reference: https://stackoverflow.com/questions/2075912/generate-a-random-binary-number-with-a-variable-proportion-of-1-bits
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
	for {
		select {
		case <-stub.Interrupt:
			return
		case c := <-stub.Input:
			if c == nil {
				stub.Close()
				return
			}
			t.corrupt(c.Data)
			stub.Output <- c
		}
	}
}

func init() {
	Register("corrupt", new(CorruptToxic))
}
