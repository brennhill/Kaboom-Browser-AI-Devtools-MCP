// Purpose: Tests for automatic noise rule detection on connect.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// noise_autorun_test.go — Tests for automatic noise detection after navigation.
package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// ============================================
// noiseAutoRunner Tests
// ============================================

func TestNoiseAutoRunner_ScheduleRunsOnce(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := newNoiseAutoRunner(func() {
		runCount.Add(1)
	}, 50*time.Millisecond)

	runner.schedule()

	// Wait for debounce + execution
	time.Sleep(150 * time.Millisecond)

	if got := runCount.Load(); got != 1 {
		t.Errorf("run count = %d, want 1", got)
	}
}

// Exercises the debounce decision directly against the pure planRunSchedule seam
// with a controlled clock. The previous sleep-based version raced: schedule() runs
// the first invocation on a goroutine, and under load that goroutine could finish
// between the rapid calls, clearing `pending` and letting a second run be queued.
func TestNoiseAutoRunner_DebouncesRapidSchedules(t *testing.T) {
	t.Parallel()

	const interval = 100 * time.Millisecond
	runner := newNoiseAutoRunner(func() {}, interval)
	now := time.Now()

	// First schedule: lastRun is zero, so the interval has "elapsed" — run now
	// and mark the runner pending.
	immediate, _, shouldRun := runner.planRunSchedule(now)
	if !shouldRun || !immediate {
		t.Fatalf("first schedule: immediate=%v shouldRun=%v, want true/true", immediate, shouldRun)
	}

	// Rapid follow-up schedules coalesce into the pending run — no extra work.
	for i := 2; i <= 5; i++ {
		if _, _, again := runner.planRunSchedule(now.Add(time.Duration(i) * time.Millisecond)); again {
			t.Fatalf("schedule #%d arrived while a run was pending: want coalesced (shouldRun=false)", i)
		}
	}
}

// After a run completes, a schedule inside the interval must be deferred by the
// remaining time rather than firing immediately.
func TestNoiseAutoRunner_DefersScheduleWithinInterval(t *testing.T) {
	t.Parallel()

	const interval = 100 * time.Millisecond
	runner := newNoiseAutoRunner(func() {}, interval)

	start := time.Now()
	runner.lastRun = start
	runner.pending = false

	immediate, delay, shouldRun := runner.planRunSchedule(start.Add(30 * time.Millisecond))
	if !shouldRun {
		t.Fatal("schedule after a completed run should be accepted")
	}
	if immediate {
		t.Error("schedule inside the debounce interval should not run immediately")
	}
	if want := 70 * time.Millisecond; delay != want {
		t.Errorf("delay = %v, want %v (remaining debounce)", delay, want)
	}
}

func TestNoiseAutoRunner_RunsAgainAfterDebounceExpires(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := newNoiseAutoRunner(func() {
		runCount.Add(1)
	}, 50*time.Millisecond)

	runner.schedule()
	time.Sleep(100 * time.Millisecond) // Wait for first run

	runner.schedule()
	time.Sleep(100 * time.Millisecond) // Wait for second run

	if got := runCount.Load(); got != 2 {
		t.Errorf("run count = %d, want 2 (one per debounce window)", got)
	}
}

func TestNoiseAutoRunner_NilFuncDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Should not panic with nil function
	runner := newNoiseAutoRunner(nil, 50*time.Millisecond)
	runner.schedule() // Should be a no-op
	time.Sleep(100 * time.Millisecond)
}

func TestNoiseAutoDetectEnabled_DefaultOff(t *testing.T) {
	t.Setenv(noiseAutoDetectEnvVar, "")

	if noiseAutoDetectEnabled() {
		t.Fatal("noise auto-detect should default to off")
	}
}

func TestNoiseAutoDetectEnabled_TruthyValues(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE", "on", "yes"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv(noiseAutoDetectEnvVar, val)
			if !noiseAutoDetectEnabled() {
				t.Fatalf("expected %q to enable noise auto-detect", val)
			}
		})
	}
}
