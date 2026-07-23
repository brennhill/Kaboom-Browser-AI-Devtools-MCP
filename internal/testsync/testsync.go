// Purpose: Deterministic waiting for tests — poll a condition instead of sleeping a guess.
// Docs: docs/core/testing-guidelines.md

// testsync.go — Shared `Eventually` helpers for tests that wait on async work.
//
// Repo rule 9 prefers controlled waiting over sleep-based timing. A fixed
// `time.Sleep` before an assertion encodes a guess about how long background
// work takes, and it is wrong in both directions: too short and the test flakes
// under load, too long and every run pays the worst case. Polling a condition
// with a generous deadline is both faster in the common case and stable in the
// slow case.
//
// This package is test-only. Nothing in production code may import it — it pulls
// in `testing`, which registers test flags on any binary that links it.
package testsync

import (
	"runtime"
	"testing"
	"time"
)

// DefaultTimeout is the deadline for waits with no specific budget. It is
// deliberately generous: it bounds a hang, it does not assert latency. Tests
// that need to assert how *fast* something happens must measure elapsed time
// explicitly rather than relying on a tight timeout.
const DefaultTimeout = 5 * time.Second

// pollInterval trades a little CPU for responsiveness. At 2ms a condition that
// becomes true immediately costs ~2ms instead of the full sleep it replaced.
const pollInterval = 2 * time.Millisecond

// Eventually blocks until cond returns true, then returns. If the deadline
// passes first it fails the test with msg.
//
// cond must be safe to call repeatedly and from the calling goroutine; guard
// shared state with a mutex or use atomics.
func Eventually(tb testing.TB, timeout time.Duration, msg string, cond func() bool) {
	tb.Helper()
	if waitFor(timeout, cond) {
		return
	}
	tb.Fatalf("timed out after %v waiting for %s", timeout, msg)
}

// EventuallyNoFail is Eventually without the failure: it reports whether cond
// became true. Use it when the caller wants to assert something more specific
// about the end state than "it happened".
func EventuallyNoFail(timeout time.Duration, cond func() bool) bool {
	return waitFor(timeout, cond)
}

// Value blocks until get returns ok, then returns the value. On timeout it
// fails the test with msg and returns the zero value.
func Value[T any](tb testing.TB, timeout time.Duration, msg string, get func() (T, bool)) T {
	tb.Helper()
	var result T
	found := waitFor(timeout, func() bool {
		v, ok := get()
		if ok {
			result = v
		}
		return ok
	})
	if !found {
		tb.Fatalf("timed out after %v waiting for %s", timeout, msg)
	}
	return result
}

// gcPollInterval is coarser than pollInterval because each poll forces a GC.
const gcPollInterval = 20 * time.Millisecond

// SettledGoroutines returns the live goroutine count once it has stopped
// changing, so a leak baseline is not sampled while unrelated goroutines from a
// previous test are still winding down. Falls back to the last reading if the
// count never settles.
func SettledGoroutines() int {
	last := -1
	stable := 0
	deadline := time.Now().Add(DefaultTimeout)
	for time.Now().Before(deadline) {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n == last {
			// Two consecutive identical readings across a GC: treat as settled.
			if stable++; stable >= 2 {
				return n
			}
		} else {
			stable = 0
			last = n
		}
		time.Sleep(gcPollInterval)
	}
	return last
}

// EventuallyGoroutines waits for the live goroutine count to settle at or below
// max, forcing a GC each poll so finished goroutines are actually reaped.
//
// The pattern it replaces — sleep, GC, sleep, then assert a count — is a
// perennial flake: goroutine teardown is not synchronous with Close(), so the
// sleep is a guess about scheduler timing. Polling both removes the guess and
// returns as soon as teardown completes.
func EventuallyGoroutines(tb testing.TB, max int, msg string) {
	tb.Helper()
	last := runtime.NumGoroutine()
	ok := waitForInterval(DefaultTimeout, gcPollInterval, func() bool {
		runtime.GC()
		last = runtime.NumGoroutine()
		return last <= max
	})
	if !ok {
		tb.Errorf("goroutine leak waiting for %s: count settled at %d, want <= %d", msg, last, max)
	}
}

func waitFor(timeout time.Duration, cond func() bool) bool {
	return waitForInterval(timeout, pollInterval, cond)
}

func waitForInterval(timeout, interval time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			// One last check: cond may have flipped during the final sleep.
			return cond()
		}
		time.Sleep(interval)
	}
}
