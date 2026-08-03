// Purpose: Tests for automatic noise rule detection on connect.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// noise_autorun_test.go — Tests for automatic noise detection after navigation.
package noiseautorun

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
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

func TestWireFirstConnectRunsOnceAndHonorsShutdown(t *testing.T) {
	for name, shutDown := range map[string]bool{"connected": false, "shutdown": true} {
		t.Run(name, func(t *testing.T) {
			cap := capture.NewCapture()
			defer cap.Close()
			shutdown := make(chan struct{})
			if shutDown {
				close(shutdown)
			}
			var calls atomic.Int32
			WireFirstConnect(cap, shutdown, func() { calls.Add(1) })
			cap.Lifecycle().Emit(lifecycle.EventExtensionDisconnected, nil)
			cap.Lifecycle().Emit(lifecycle.EventExtensionConnected, nil)
			cap.Lifecycle().Emit(lifecycle.EventExtensionConnected, nil)
			time.Sleep(50 * time.Millisecond)
			want := int32(1)
			if shutDown {
				want = 0
			}
			if got := calls.Load(); got != want {
				t.Fatalf("first-connect calls = %d, want %d", got, want)
			}
		})
	}
	WireFirstConnect(nil, nil, func() {})
	unused := capture.NewCapture()
	defer unused.Close()
	WireFirstConnect(unused, nil, nil)
}

func TestWireNavigationAndDetectApplyHighConfidenceRules(t *testing.T) {
	t.Setenv(EnvVar, "true")
	cap := capture.NewCapture()
	defer cap.Close()
	called := make(chan struct{}, 1)
	WireNavigation(cap, func() { called <- struct{}{} })
	cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "navigation", Timestamp: time.Now().UnixMilli()}})
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("navigation did not schedule noise detection")
	}
	config := noise.NewNoiseConfig()
	logs := make([]types.LogEntry, 30)
	for index := range logs {
		logs[index] = types.LogEntry{"message": "repeated application heartbeat"}
	}
	Detect(config, cap, logs)
	if !config.IsConsoleNoise(types.LogEntry{"message": "repeated application heartbeat"}) {
		t.Fatal("high-confidence repetitive message was not classified as noise")
	}
	Detect(nil, cap, logs)
	Detect(config, nil, logs)
}
