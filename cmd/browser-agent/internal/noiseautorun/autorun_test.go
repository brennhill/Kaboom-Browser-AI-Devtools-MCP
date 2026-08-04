// Purpose: Tests for automatic noise rule detection on connect.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// noise_autorun_test.go — Tests for automatic noise detection after navigation.
package noiseautorun

import (
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

	now := time.Unix(100, 0)
	var dispatched []func()
	runCount := 0
	runner := newRunner(func() { runCount++ }, 50*time.Millisecond, runnerRuntime{
		now:      func() time.Time { return now },
		dispatch: func(run func()) { dispatched = append(dispatched, run) },
		delay:    func(_ time.Duration, run func()) { dispatched = append(dispatched, run) },
	})

	runner.Schedule()
	if len(dispatched) != 1 {
		t.Fatalf("dispatched callbacks = %d, want 1", len(dispatched))
	}
	dispatched[0]()

	if got := runCount; got != 1 {
		t.Errorf("run count = %d, want 1", got)
	}
}

func TestNoiseAutoRunner_DebouncesRapidSchedules(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	var dispatched []func()
	runCount := 0
	runner := newRunner(func() { runCount++ }, 100*time.Millisecond, runnerRuntime{
		now:      func() time.Time { return now },
		dispatch: func(run func()) { dispatched = append(dispatched, run) },
		delay:    func(_ time.Duration, run func()) { dispatched = append(dispatched, run) },
	})

	// Schedule 5 times rapidly — should only run once within debounce window
	for i := 0; i < 5; i++ {
		runner.Schedule()
	}

	if len(dispatched) != 1 {
		t.Fatalf("dispatched callbacks = %d, want one coalesced run", len(dispatched))
	}
	dispatched[0]()

	if got := runCount; got != 1 {
		t.Errorf("run count after rapid schedules = %d, want 1", got)
	}
}

func TestNoiseAutoRunner_RunsAgainAfterDebounceExpires(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	var immediate []func()
	var delayed []func()
	var delays []time.Duration
	runCount := 0
	runner := newRunner(func() { runCount++ }, 50*time.Millisecond, runnerRuntime{
		now:      func() time.Time { return now },
		dispatch: func(run func()) { immediate = append(immediate, run) },
		delay: func(delay time.Duration, run func()) {
			delays = append(delays, delay)
			delayed = append(delayed, run)
		},
	})

	runner.Schedule()
	immediate[0]()

	now = now.Add(20 * time.Millisecond)
	runner.Schedule()
	if len(delayed) != 1 || len(delays) != 1 || delays[0] != 30*time.Millisecond {
		t.Fatalf("delayed schedule = (%d callbacks, %v), want one callback after 30ms", len(delayed), delays)
	}
	now = now.Add(30 * time.Millisecond)
	delayed[0]()

	if got := runCount; got != 2 {
		t.Errorf("run count = %d, want 2 (one per debounce window)", got)
	}
}

func TestNoiseAutoRunner_NilFuncDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Should not panic with nil function
	runner := NewRunner(nil, 50*time.Millisecond)
	runner.Schedule() // Should be a no-op
}

func TestNoiseAutoRunner_PanicDoesNotLeaveRunnerPending(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	var dispatched []func()
	runner := newRunner(func() { panic("detector failed") }, time.Second, runnerRuntime{
		now:      func() time.Time { return now },
		dispatch: func(run func()) { dispatched = append(dispatched, run) },
		delay:    func(_ time.Duration, run func()) { dispatched = append(dispatched, run) },
	})

	runner.Schedule()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		dispatched[0]()
	}()
	if recovered != "detector failed" {
		t.Fatalf("recovered panic = %v, want detector failure", recovered)
	}

	now = now.Add(time.Second)
	runner.Schedule()
	if len(dispatched) != 2 {
		t.Fatalf("dispatched callbacks = %d, want runner to recover scheduling after panic", len(dispatched))
	}
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
			timer := make(chan time.Time, 1)
			done := make(chan struct{})
			calls := 0
			wireFirstConnect(cap, shutdown, func() { calls++ }, firstConnectRuntime{
				launch: func(run func()) { go func() { defer close(done); run() }() },
				after:  func(time.Duration) <-chan time.Time { return timer },
			})
			cap.Lifecycle().Emit(lifecycle.EventExtensionDisconnected, nil)
			cap.Lifecycle().Emit(lifecycle.EventExtensionConnected, nil)
			cap.Lifecycle().Emit(lifecycle.EventExtensionConnected, nil)
			if !shutDown {
				timer <- time.Unix(101, 0)
			}
			<-done
			want := 1
			if shutDown {
				want = 0
			}
			if got := calls; got != want {
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
