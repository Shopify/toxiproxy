package testhelper_test

import (
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/testhelper"
)

func TestTimeoutAfter(t *testing.T) {
	err := testhelper.TimeoutAfter(5*time.Millisecond, func() {})
	if err != nil {
		t.Fatal("Non blocking function should not timeout.")
	}

	err = testhelper.TimeoutAfter(5*time.Millisecond, func() {
		time.Sleep(time.Second)
	})
	if err == nil {
		t.Fatal("Blocking function should timeout.")
	}
}
