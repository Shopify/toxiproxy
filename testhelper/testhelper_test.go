package testhelper

import (
  "testing"
  "time"
)

func TestTimeoutAfter(t *testing.T) {
  err := TimeoutAfter(5*time.Millisecond, func() {})
  if err != nil {
    t.Fatal("Non blocking function should not timeout.")
  }

  err = TimeoutAfter(5*time.Millisecond, func() {
    time.Sleep(time.Second)
  })
  if err == nil {
    t.Fatal("Blocking function should timeout.")
  }
}
