// Purpose: Tests for automatic noise rule detection on connect.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// noise_autorun_test.go — Tests for automatic noise detection after navigation.
package noiseautorun

import (
	"sync/atomic"
	"testing"
	"time"
)

// ============================================
// Runner tests
// ============================================

func TestNoiseAutoRunner_ScheduleRunsOnce(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := NewRunner(func() {
		runCount.Add(1)
	}, 50*time.Millisecond)

	runner.Schedule()

	// Wait for debounce + execution
	time.Sleep(150 * time.Millisecond)

	if got := runCount.Load(); got != 1 {
		t.Errorf("run count = %d, want 1", got)
	}
}

func TestNoiseAutoRunner_DebouncesRapidSchedules(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := NewRunner(func() {
		runCount.Add(1)
	}, 100*time.Millisecond)

	// Schedule 5 times rapidly — should only run once within debounce window
	for i := 0; i < 5; i++ {
		runner.Schedule()
	}

	// Wait for debounce + execution
	time.Sleep(250 * time.Millisecond)

	if got := runCount.Load(); got != 1 {
		t.Errorf("run count after rapid schedules = %d, want 1", got)
	}
}

func TestNoiseAutoRunner_RunsAgainAfterDebounceExpires(t *testing.T) {
	t.Parallel()

	var runCount atomic.Int32
	runner := NewRunner(func() {
		runCount.Add(1)
	}, 50*time.Millisecond)

	runner.Schedule()
	time.Sleep(100 * time.Millisecond) // Wait for first run

	runner.Schedule()
	time.Sleep(100 * time.Millisecond) // Wait for second run

	if got := runCount.Load(); got != 2 {
		t.Errorf("run count = %d, want 2 (one per debounce window)", got)
	}
}

func TestNoiseAutoRunner_NilFuncDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Should not panic with nil function
	runner := NewRunner(nil, 50*time.Millisecond)
	runner.Schedule() // Should be a no-op
	time.Sleep(100 * time.Millisecond)
}

func TestNoiseAutoDetectEnabled_DefaultOff(t *testing.T) {
	t.Setenv(EnvVar, "")

	if Enabled() {
		t.Fatal("noise auto-detect should default to off")
	}
}

func TestNoiseAutoDetectEnabled_TruthyValues(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE", "on", "yes"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv(EnvVar, val)
			if !Enabled() {
				t.Fatalf("expected %q to enable noise auto-detect", val)
			}
		})
	}
}
