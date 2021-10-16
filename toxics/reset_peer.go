package toxics

import (
	"time"
)

/*
The ResetToxic sends closes the connection abruptly after a timeout (in ms).
The behavior of Close is set to discard any unsent/unacknowledged data by setting SetLinger to 0,
~= sets TCP RST flag and resets the connection.
If the timeout is set to 0, then the connection will be reset immediately.

Drop data since it will initiate a graceful close by sending the FIN/ACK. (io.EOF)
*/

type ResetToxic struct {
	// Timeout in milliseconds
	Timeout int64 `json:"timeout"`
}

func (t *ResetToxic) Pipe(stub *ToxicStub) {
	timeout := time.Duration(t.Timeout) * time.Millisecond

	for {
		select {
		case <-stub.Interrupt:
			return
		case <-stub.Input:
			<-time.After(timeout)
			stub.Close()
			return
		}
	}
}

func init() {
	Register("reset_peer", new(ResetToxic))
}
