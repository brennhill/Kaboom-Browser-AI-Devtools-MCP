// usage_counter_test.go — Tests for aggregated tool usage counters.

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestUsageTracker_Increment(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("observe:errors", 0, false)

	counts := c.Peek()
	if counts["observe:errors"] != 3 {
		t.Fatalf("count = %d, want 3", counts["observe:errors"])
	}
}

func TestUsageTracker_SwapAndReset(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("interact:click", 0, false)

	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if len(snapshot.ToolStats) != 2 {
		t.Fatalf("ToolStats length = %d, want 2", len(snapshot.ToolStats))
	}

	// After swap, should be empty.
	fresh := c.SwapAndReset()
	if fresh != nil {
		t.Fatalf("second SwapAndReset returned %+v, want nil", fresh)
	}
}

func TestUsageTracker_ConcurrentIncrement(t *testing.T) {
	c := NewUsageTracker()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			c.RecordToolCall("concurrent:key", 0, false)
		}()
	}
	wg.Wait()

	counts := c.Peek()
	if counts["concurrent:key"] != goroutines {
		t.Fatalf("count = %d, want %d", counts["concurrent:key"], goroutines)
	}
}

func TestUsageTracker_ConcurrentSwapAndIncrement(t *testing.T) {
	c := NewUsageTracker()
	const incrementors = 100
	const incrementsEach = 50

	var wg sync.WaitGroup
	var swapMu sync.Mutex

	// Start incrementor goroutines.
	wg.Add(incrementors)
	for i := 0; i < incrementors; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsEach; j++ {
				c.RecordToolCall("key", 0, false)
			}
		}()
	}

	// Start a swapper goroutine that runs concurrently with incrementors.
	stopSwapper := make(chan struct{})
	swapperDone := make(chan struct{})
	var snapshots []*UsageSnapshot
	go func() {
		defer close(swapperDone)
		for {
			select {
			case <-stopSwapper:
				return
			default:
				snapshot := c.SwapAndReset()
				if snapshot != nil {
					swapMu.Lock()
					snapshots = append(snapshots, snapshot)
					swapMu.Unlock()
				}
				runtime.Gosched() // yield to avoid burning CPU in tight loop
			}
		}
	}()

	// Wait for all incrementors to finish.
	wg.Wait()

	// Signal the swapper to stop.
	close(stopSwapper)
	<-swapperDone

	// Collect the final snapshot.
	if final := c.SwapAndReset(); final != nil {
		snapshots = append(snapshots, final)
	}

	// Sum all counts across all snapshots.
	total := 0
	for _, snap := range snapshots {
		for _, stat := range snap.ToolStats {
			if stat.Tool == "key" {
				total += stat.Count
			}
		}
	}

	expected := incrementors * incrementsEach
	if total != expected {
		t.Fatalf("total count = %d, want %d (counts were lost)", total, expected)
	}
}

func TestUsageTracker_Peek(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 0, false)
	c.RecordToolCall("observe:page", 0, false)
	c.RecordToolCall("interact:click", 0, false)

	peeked := c.Peek()
	if peeked["observe:page"] != 2 {
		t.Fatalf("peeked observe:page = %d, want 2", peeked["observe:page"])
	}
	if peeked["interact:click"] != 1 {
		t.Fatalf("peeked interact:click = %d, want 1", peeked["interact:click"])
	}

	// Peek should not reset — counts should still be there.
	peeked2 := c.Peek()
	if peeked2["observe:page"] != 2 {
		t.Fatalf("second peek observe:page = %d, want 2 (Peek should not reset)", peeked2["observe:page"])
	}

	// Mutating the returned map should not affect the counter.
	peeked["observe:page"] = 999
	peeked3 := c.Peek()
	if peeked3["observe:page"] != 2 {
		t.Fatalf("peek after mutation = %d, want 2 (returned map should be a copy)", peeked3["observe:page"])
	}
}

func TestUsageTracker_PeekEmpty(t *testing.T) {
	c := NewUsageTracker()
	peeked := c.Peek()
	if len(peeked) != 0 {
		t.Fatalf("Peek on new counter returned %d entries, want 0", len(peeked))
	}
}

func TestUsageTracker_SwapAndResetEmpty(t *testing.T) {
	c := NewUsageTracker()
	snapshot := c.SwapAndReset()
	if snapshot != nil {
		t.Fatalf("SwapAndReset on empty tracker returned %+v, want nil", snapshot)
	}
}

func TestUsageTracker_RecordToolCallWithLatency(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 50*time.Millisecond, false)
	c.RecordToolCall("observe:page", 150*time.Millisecond, false)
	c.RecordToolCall("interact:click", 30*time.Millisecond, false)

	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	for _, s := range snapshot.ToolStats {
		if s.Tool == "observe:page" {
			if s.Count != 2 {
				t.Fatalf("observe:page count = %d, want 2", s.Count)
			}
			if s.LatencyAvgMs != 100 {
				t.Fatalf("observe:page lat_avg = %d, want 100", s.LatencyAvgMs)
			}
			if s.LatencyMaxMs != 150 {
				t.Fatalf("observe:page lat_max = %d, want 150", s.LatencyMaxMs)
			}
		}
		if s.Tool == "interact:click" {
			if s.LatencyAvgMs != 30 {
				t.Fatalf("interact:click lat_avg = %d, want 30", s.LatencyAvgMs)
			}
		}
	}
}

func TestUsageTracker_RecordToolCallError(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 0, false)
	c.RecordToolCall("observe:page", 0, true) // error

	snapshot := c.Peek()
	if snapshot["observe:page"] != 2 {
		t.Fatalf("observe:page = %d, want 2", snapshot["observe:page"])
	}
	if snapshot["err:observe:page"] != 1 {
		t.Fatalf("err:observe:page = %d, want 1", snapshot["err:observe:page"])
	}
}

func TestUsageTracker_RecordAsyncOutcome(t *testing.T) {
	c := NewUsageTracker()
	c.RecordAsyncOutcome("complete")
	c.RecordAsyncOutcome("complete")
	c.RecordAsyncOutcome("error")
	c.RecordAsyncOutcome("timeout")
	c.RecordAsyncOutcome("expired")

	snapshot := c.Peek()
	if snapshot["async:complete"] != 2 {
		t.Fatalf("async:complete = %d, want 2", snapshot["async:complete"])
	}
	if snapshot["async:timeout"] != 1 {
		t.Fatalf("async:timeout = %d, want 1", snapshot["async:timeout"])
	}
	if snapshot["async:error"] != 1 {
		t.Fatalf("async:error = %d, want 1", snapshot["async:error"])
	}
	if snapshot["async:expired"] != 1 {
		t.Fatalf("async:expired = %d, want 1", snapshot["async:expired"])
	}
}

func TestUsageTracker_FirstToolCallOnlyOncePerInstall(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	resetInstallIDState()
	resetFirstToolCallState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetFirstToolCallState()
		resetKaboomDir()
	})

	received := make(chan map[string]any, 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	firstRunEvents := map[string]map[string]any{}
	for len(firstRunEvents) < 3 {
		select {
		case body := <-received:
			if event, ok := body["event"].(string); ok {
				firstRunEvents[event] = body
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for first-run telemetry, got %v", firstRunEvents)
		}
	}

	sessionStart := firstRunEvents["session_start"]
	if sessionStart["reason"] != "first_activity" {
		t.Fatalf("session_start reason = %v, want first_activity", sessionStart["reason"])
	}
	if _, ok := firstRunEvents["tool_call"]; !ok {
		t.Fatal("missing tool_call beacon on first run")
	}
	if _, ok := firstRunEvents["first_tool_call"]; !ok {
		t.Fatal("missing first_tool_call beacon on first run")
	}

	resetInstallIDState()
	resetFirstToolCallState()

	trackerAfterRestart := NewUsageTracker()
	trackerAfterRestart.RecordToolCall("observe:page", 0, false)

	secondRunEvents := map[string]map[string]any{}
	for len(secondRunEvents) < 2 {
		select {
		case body := <-received:
			if event, ok := body["event"].(string); ok {
				secondRunEvents[event] = body
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for second-run telemetry, got %v", secondRunEvents)
		}
	}

	sessionStartAfterRestart := secondRunEvents["session_start"]
	if sessionStartAfterRestart["reason"] != "first_activity" {
		t.Fatalf(
			"session_start after restart reason = %v, want first_activity",
			sessionStartAfterRestart["reason"],
		)
	}
	if _, ok := secondRunEvents["tool_call"]; !ok {
		t.Fatal("missing tool_call beacon after restart")
	}
	if _, ok := secondRunEvents["first_tool_call"]; ok {
		t.Fatal("unexpected duplicate first_tool_call after restart for same install")
	}

	select {
	case body := <-received:
		if body["event"] == "first_tool_call" {
			t.Fatal("unexpected duplicate first_tool_call after restart for same install")
		}
	case <-time.After(200 * time.Millisecond):
	}
}

func TestUsageTracker_SessionStartOnlyOnFirstCall(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	received := make(chan map[string]any, 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("a", 0, false)
	tracker.RecordToolCall("b", 0, false)

	// Drain all events.
	sessionStarts := 0
	deadline := time.After(2 * time.Second)
	for {
		select {
		case body := <-received:
			if body["event"] == "session_start" {
				sessionStarts++
				if body["reason"] != "first_activity" {
					t.Fatalf("session_start reason = %v, want first_activity", body["reason"])
				}
			}
		case <-deadline:
			goto done
		}
	}
done:
	if sessionStarts != 1 {
		t.Fatalf("session_start fired %d times, want exactly 1", sessionStarts)
	}
}

func TestTouchSession_TimeoutEmitsSessionEnd(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	received := make(chan map[string]any, 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// TouchSession should detect expiry and fire session_end via callback.
	TouchSession()

	// Drain events looking for session_end.
	for {
		select {
		case body := <-received:
			if body["event"] == "session_end" {
				if body["reason"] != "timeout" {
					t.Errorf("session_end reason = %v, want timeout", body["reason"])
				}
				goto found
			}
		case <-time.After(3 * time.Second):
			t.Fatal("session_end(timeout) beacon not received")
		}
	}
found:
}

func TestEmitSessionEnd_NoOpWhenNoCalls(t *testing.T) {
	tracker := NewUsageTracker()

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	tracker.EmitSessionEnd("test")
	time.Sleep(50 * time.Millisecond)
	if called {
		t.Fatal("EmitSessionEnd should not fire when no calls were made")
	}
}

func TestSessionDurationSeconds(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name  string
		start time.Time
		now   time.Time
		want  int64
	}{
		{"zero start -> 0", time.Time{}, base, 0},
		{"same instant -> 0", base, base, 0},
		{"sub-second -> 0 (truncates)", base, base.Add(900 * time.Millisecond), 0},
		{"42s -> 42", base, base.Add(42 * time.Second), 42},
		{"clock skew (now < start) -> 0, not negative", base, base.Add(-5 * time.Second), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionDurationSeconds(tc.start, tc.now); got != tc.want {
				t.Errorf("sessionDurationSeconds = %d, want %d", got, tc.want)
			}
		})
	}
}

// The injected clock makes session_end duration deterministic — no wall-clock
// dependence, no sleeps.
func TestEmitSessionEnd_DurationFromInjectedClock(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	received := make(chan map[string]any, 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	base := time.Unix(1_700_000_000, 0)
	tracker := NewUsageTracker()
	tracker.now = func() time.Time { return base.Add(42 * time.Second) } // "now" at end
	tracker.mu.Lock()
	tracker.sessionStart = base
	tracker.sessionCalls = 3
	tracker.mu.Unlock()

	tracker.EmitSessionEnd("timeout")

	for {
		select {
		case body := <-received:
			if body["event"] != "session_end" {
				continue
			}
			// JSON numbers decode as float64.
			if body["duration_s"] != float64(42) {
				t.Fatalf("duration_s = %v, want 42", body["duration_s"])
			}
			if body["tool_calls"] != float64(3) {
				t.Fatalf("tool_calls = %v, want 3", body["tool_calls"])
			}
			return
		case <-time.After(3 * time.Second):
			t.Fatal("session_end beacon not received")
		}
	}
}

func TestAppError_PayloadStructure(t *testing.T) {
	drainSem()
	received := make(chan map[string]any, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	AppError("daemon_panic", map[string]string{"extra": "info"})

	for {
		select {
		case body := <-received:
			if body["event"] == "app_error" {
				if body["error_kind"] != "internal" {
					t.Errorf("error_kind = %v, want internal", body["error_kind"])
				}
				if body["error_code"] != "DAEMON_PANIC" {
					t.Errorf("error_code = %v, want DAEMON_PANIC", body["error_code"])
				}
				if body["severity"] != "fatal" {
					t.Errorf("severity = %v, want fatal", body["severity"])
				}
				if body["source"] != "daemon" {
					t.Errorf("source = %v, want daemon", body["source"])
				}
				if _, exists := body["detail"]; exists {
					t.Error("detail field should not be present — not in Counterscale contract")
				}
				if body["extra"] != "info" {
					t.Errorf("extra = %v, want info", body["extra"])
				}
				if _, ok := body["iid"].(string); !ok {
					t.Error("missing iid")
				}
				if _, ok := body["ts"].(string); !ok {
					t.Error("missing ts")
				}
				goto done
			}
		case <-time.After(2 * time.Second):
			t.Fatal("app_error beacon not received")
		}
	}
done:
}

func TestAppError_NilProps(t *testing.T) {
	drainSem()
	received := make(chan map[string]any, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	AppError("test_error", nil) // nil props should not panic

	for {
		select {
		case body := <-received:
			if body["event"] == "app_error" {
				goto done
			}
		case <-time.After(2 * time.Second):
			t.Fatal("app_error beacon not received")
		}
	}
done:
}

func TestUsageTracker_SessionDepth(t *testing.T) {
	c := NewUsageTracker()
	if c.SessionDepth() != 0 {
		t.Fatalf("initial session depth = %d, want 0", c.SessionDepth())
	}

	c.RecordToolCall("a", 0, false)
	c.RecordToolCall("b", 0, false)
	c.RecordToolCall("c", time.Millisecond, false)

	if c.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", c.SessionDepth())
	}

	// SwapAndReset should not reset session depth (it's a session-scoped counter).
	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if c.SessionDepth() != 3 {
		t.Fatalf("session depth after swap = %d, want 3 (should not reset)", c.SessionDepth())
	}

	// Further calls add to the running total.
	c.RecordToolCall("d", 0, false)
	if c.SessionDepth() != 4 {
		t.Fatalf("session depth after +1 = %d, want 4", c.SessionDepth())
	}
}

func TestUsageTracker_LatencyNotIncludedWhenNoLatencyRecorded(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 0, false) // no latency variant

	snapshot := c.Peek()
	if _, exists := snapshot["lat_avg:observe:page"]; exists {
		t.Fatal("lat_avg should not exist when no latency was recorded")
	}
}

func TestUsageTracker_MultipleKeys(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("interact:click", 0, false)
	c.RecordToolCall("analyze:performance", 0, false)
	c.RecordToolCall("analyze:performance", 0, false)
	c.RecordToolCall("analyze:performance", 0, false)

	counts := c.Peek()
	if counts["observe:errors"] != 2 {
		t.Fatalf("observe:errors = %d, want 2", counts["observe:errors"])
	}
	if counts["interact:click"] != 1 {
		t.Fatalf("interact:click = %d, want 1", counts["interact:click"])
	}
	if counts["analyze:performance"] != 3 {
		t.Fatalf("analyze:performance = %d, want 3", counts["analyze:performance"])
	}
}
