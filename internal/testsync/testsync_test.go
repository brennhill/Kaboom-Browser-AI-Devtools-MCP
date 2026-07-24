// testsync_test.go — Tests for the shared Eventually helpers.

package testsync

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventuallyReturnsAsSoonAsConditionHolds(t *testing.T) {
	var flipped atomic.Bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		flipped.Store(true)
	}()

	start := time.Now()
	Eventually(t, time.Second, "flag to flip", flipped.Load)
	elapsed := time.Since(start)

	// The point of polling: a 10ms condition costs ~10ms, not the full budget.
	if elapsed > 500*time.Millisecond {
		t.Errorf("Eventually took %v; polling should return promptly after the condition holds", elapsed)
	}
}

func TestEventuallyReturnsImmediatelyWhenAlreadyTrue(t *testing.T) {
	start := time.Now()
	Eventually(t, time.Second, "already true", func() bool { return true })
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Eventually took %v for an already-true condition", elapsed)
	}
}

func TestEventuallyNoFailReportsTimeout(t *testing.T) {
	if EventuallyNoFail(20*time.Millisecond, func() bool { return false }) {
		t.Fatal("expected false when the condition never holds")
	}
}

func TestValueReturnsTheProducedValue(t *testing.T) {
	var counter atomic.Int32
	got := Value(t, time.Second, "counter to reach 3", func() (int32, bool) {
		v := counter.Add(1)
		return v, v >= 3
	})
	if got != 3 {
		t.Errorf("Value = %d, want 3", got)
	}
}

func TestWaitForRechecksAfterTheDeadline(t *testing.T) {
	// A condition that only becomes true at the very end must still be seen —
	// otherwise the helper reintroduces the race it exists to remove.
	var calls atomic.Int32
	ok := waitFor(10*time.Millisecond, func() bool { return calls.Add(1) > 2 })
	if !ok {
		t.Fatal("waitFor must re-check the condition once more after the deadline")
	}
}

func TestEventuallyGoroutinesWaitsForTeardown(t *testing.T) {
	runtime.GC()
	baseline := runtime.NumGoroutine()

	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() { <-stop }()
	}
	// Release them asynchronously; the helper must wait rather than sample once.
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(stop)
	}()

	EventuallyGoroutines(t, baseline+1, "spawned goroutines to exit")
}
