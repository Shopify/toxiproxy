package testhelper

import (
	"time"
  "fmt"
)

func TimeoutAfter(after time.Duration, f func()) error {
	success := make(chan struct{})
	go func() {
		f()
		close(success)
	}()
	select {
	case <-success:
		return nil
	case <-time.After(after):
		return fmt.Errorf("timed out after %s", after)
	}
}
