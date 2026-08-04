// e2e_usage_summary_test.go — Tests app-telemetry usage aggregation and reset.
// Docs: docs/features/feature/app-telemetry/index.md

package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestE2E_UsageSummary_FullPayload(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{
			{Tool: "observe:page", Family: "observe", Name: "page", Count: 10, ErrorCount: 1, LatencyAvgMs: 42, LatencyMaxMs: 200},
			{Tool: "interact:click", Family: "interact", Name: "click", Count: 3},
		},
		AsyncOutcomes: map[string]int{"complete": 5, "timeout": 2},
	}
	BeaconUsageSummary(5, snapshot)

	body := waitForEvent(t, received, "usage_summary")
	requireEnvelope(t, body, "usage_summary")

	if wm, ok := body["window_m"].(float64); !ok || wm != 5 {
		t.Errorf("window_m = %v, want 5", body["window_m"])
	}

	// tool_stats: check it's a non-empty array.
	statsRaw, ok := body["tool_stats"]
	if !ok {
		t.Fatal("missing tool_stats")
	}
	stats, ok := statsRaw.([]any)
	if !ok {
		t.Fatalf("tool_stats is %T, want []any", statsRaw)
	}
	if len(stats) != 2 {
		t.Fatalf("tool_stats length = %d, want 2", len(stats))
	}

	// Verify first tool_stat entry.
	entry, ok := stats[0].(map[string]any)
	if !ok {
		t.Fatalf("tool_stats[0] is %T, want map", stats[0])
	}
	if entry["tool"] != "observe:page" {
		t.Errorf("tool_stats[0].tool = %v, want observe:page", entry["tool"])
	}
	if entry["family"] != "observe" {
		t.Errorf("tool_stats[0].family = %v, want observe", entry["family"])
	}
	if cnt, ok := entry["count"].(float64); !ok || cnt != 10 {
		t.Errorf("tool_stats[0].count = %v, want 10", entry["count"])
	}
	if ec, ok := entry["error_count"].(float64); !ok || ec != 1 {
		t.Errorf("tool_stats[0].error_count = %v, want 1", entry["error_count"])
	}
	if avg, ok := entry["latency_avg_ms"].(float64); !ok || avg != 42 {
		t.Errorf("tool_stats[0].latency_avg_ms = %v, want 42", entry["latency_avg_ms"])
	}
	if max, ok := entry["latency_max_ms"].(float64); !ok || max != 200 {
		t.Errorf("tool_stats[0].latency_max_ms = %v, want 200", entry["latency_max_ms"])
	}

	// async_outcomes
	aoRaw, ok := body["async_outcomes"]
	if !ok {
		t.Fatal("missing async_outcomes")
	}
	ao, ok := aoRaw.(map[string]any)
	if !ok {
		t.Fatalf("async_outcomes is %T, want map", aoRaw)
	}
	if ao["complete"] != float64(5) {
		t.Errorf("async_outcomes.complete = %v, want 5", ao["complete"])
	}
	if ao["timeout"] != float64(2) {
		t.Errorf("async_outcomes.timeout = %v, want 2", ao["timeout"])
	}

	// Must NOT have session_depth.
	if _, exists := body["session_depth"]; exists {
		t.Error("usage_summary must not include session_depth")
	}
}

func TestE2E_UsageSummary_OmitsEmptyAsyncOutcomes(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats:     []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
		AsyncOutcomes: map[string]int{},
	}
	BeaconUsageSummary(5, snapshot)

	body := waitForEvent(t, received, "usage_summary")

	if ao, exists := body["async_outcomes"]; exists {
		if m, ok := ao.(map[string]any); ok && len(m) == 0 {
			t.Error("usage_summary should omit async_outcomes when empty")
		}
	}
}

func TestE2E_UsageSummary_NilSnapshotNoBeacon(t *testing.T) {
	received := captureBeacon(t)
	BeaconUsageSummary(5, nil)

	select {
	case body := <-received:
		t.Fatalf("nil snapshot should not fire beacon, got event=%v", body["event"])
	case <-time.After(300 * time.Millisecond):
		// Good.
	}
}

// ---------- E2E: opt-out ----------

func TestE2E_BuildUsageSummaryPayload_MatchesBeacon(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{
			{Tool: "observe:page", Family: "observe", Name: "page", Count: 5, LatencyAvgMs: 33, LatencyMaxMs: 80},
		},
		AsyncOutcomes: map[string]int{"complete": 3},
	}

	// Build the debug payload.
	payload := BuildUsageSummaryPayload(5, snapshot)
	if payload == nil {
		t.Fatal("BuildUsageSummaryPayload returned nil")
	}

	// Verify it has the same structure as what BeaconUsageSummary sends.
	if payload["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", payload["event"])
	}
	if payload["window_m"] != 5 {
		t.Errorf("window_m = %v, want 5", payload["window_m"])
	}
	if _, ok := payload["iid"].(string); !ok {
		t.Error("missing iid")
	}
	if _, ok := payload["ts"].(string); !ok {
		t.Error("missing ts")
	}

	// Fire the actual beacon and compare key fields.
	BeaconUsageSummary(5, snapshot)
	body := waitForEvent(t, received, "usage_summary")

	// JSON decodes numbers as float64; payload has int. Compare as float64.
	beaconWM, _ := body["window_m"].(float64)
	debugWM := float64(payload["window_m"].(int))
	if beaconWM != debugWM {
		t.Errorf("beacon window_m = %v, debug = %v — should match", beaconWM, debugWM)
	}
}

// ---------- E2E: SwapAndReset integration ----------

func TestE2E_SwapAndReset_AccumulatesAndResets(t *testing.T) {
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 50*time.Millisecond, false)
	tracker.RecordToolCall("observe:page", 150*time.Millisecond, false)
	tracker.RecordToolCall("observe:page", 100*time.Millisecond, true)
	tracker.RecordToolCall("interact:click", 30*time.Millisecond, false)
	tracker.RecordAsyncOutcome("complete")
	tracker.RecordAsyncOutcome("complete")
	tracker.RecordAsyncOutcome("timeout")

	snapshot := tracker.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}

	// Verify tool stats.
	if len(snapshot.ToolStats) != 2 {
		t.Fatalf("ToolStats length = %d, want 2", len(snapshot.ToolStats))
	}
	var page, click *ToolStat
	for i := range snapshot.ToolStats {
		switch snapshot.ToolStats[i].Tool {
		case "observe:page":
			page = &snapshot.ToolStats[i]
		case "interact:click":
			click = &snapshot.ToolStats[i]
		}
	}
	if page == nil {
		t.Fatal("missing observe:page in ToolStats")
	}
	if page.Count != 3 {
		t.Errorf("observe:page count = %d, want 3", page.Count)
	}
	if page.ErrorCount != 1 {
		t.Errorf("observe:page error_count = %d, want 1", page.ErrorCount)
	}
	if page.LatencyAvgMs != 100 {
		t.Errorf("observe:page latency_avg_ms = %d, want 100", page.LatencyAvgMs)
	}
	if page.LatencyMaxMs != 150 {
		t.Errorf("observe:page latency_max_ms = %d, want 150", page.LatencyMaxMs)
	}
	if click == nil || click.Count != 1 {
		t.Errorf("interact:click missing or count wrong")
	}

	// Verify async outcomes.
	if snapshot.AsyncOutcomes["complete"] != 2 {
		t.Errorf("async complete = %d, want 2", snapshot.AsyncOutcomes["complete"])
	}
	if snapshot.AsyncOutcomes["timeout"] != 1 {
		t.Errorf("async timeout = %d, want 1", snapshot.AsyncOutcomes["timeout"])
	}

	// After swap, next SwapAndReset should return nil (no new activity).
	if next := tracker.SwapAndReset(); next != nil {
		t.Errorf("second SwapAndReset should return nil, got %+v", next)
	}
}

// ---------- E2E: Warm pre-loads install ID off hot path ----------

func TestUsageBeaconLoop_FiresOnActivity(t *testing.T) {
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

	// Reset install ID state so it generates fresh for this test.
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	counter := NewUsageTracker()
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("interact:click", 0, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUsageBeaconLoopWithInterval(ctx, counter, 50*time.Millisecond)

	// Drain received until we find the usage_summary event.
	// Other events (e.g. first_tool_call) may arrive first.
	var body map[string]any
	for {
		select {
		case b := <-received:
			if b["event"] == "usage_summary" {
				body = b
				goto found
			}
		case <-time.After(2 * time.Second):
			t.Fatal("usage beacon not received within timeout")
		}
	}
found:

	if body["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", body["event"])
	}

	// window_m is top-level (JSON number → float64).
	if wm, ok := body["window_m"].(float64); !ok || wm != 0 {
		t.Errorf("window_m = %v, want 0 (sub-minute test interval)", body["window_m"])
	}

	// sid must be present (16-char hex).
	if sid, ok := body["sid"].(string); !ok || len(sid) != 16 {
		t.Errorf("sid = %v, want 16-char hex string", body["sid"])
	}

	// Verify tool_stats is an array with the expected entries.
	toolStats, ok := body["tool_stats"].([]any)
	if !ok {
		t.Fatalf("tool_stats is not an array: %T", body["tool_stats"])
	}
	if len(toolStats) == 0 {
		t.Fatal("tool_stats is empty, expected at least 1 entry")
	}
	// Verify at least one stat has observe:errors
	foundObserve := false
	for _, s := range toolStats {
		stat, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if stat["tool"] == "observe:errors" {
			foundObserve = true
			if stat["count"] != float64(2) {
				t.Errorf("observe:errors count = %v, want 2", stat["count"])
			}
		}
	}
	if !foundObserve {
		t.Errorf("tool_stats missing observe:errors entry, got %v", toolStats)
	}
}

func TestUsageBeaconLoop_SkipsWhenIdle(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Use onTick hook to wait for a known number of tick cycles.
	tickCh := make(chan struct{}, 10)
	setOnTick(func() {
		select {
		case tickCh <- struct{}{}:
		default:
		}
	})
	defer setOnTick(nil)

	counter := NewUsageTracker()
	// Don't increment — should skip.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUsageBeaconLoopWithInterval(ctx, counter, 10*time.Millisecond)

	// Wait for 3 tick cycles to complete.
	for i := 0; i < 3; i++ {
		select {
		case <-tickCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tick")
		}
	}

	cancel()

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Fatalf("beacon fired %d times, want 0 (no activity)", count)
	}
}

func TestUsageBeaconLoop_RespectsOptOut(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")

	var mu sync.Mutex
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Use onTick hook to wait for a known number of tick cycles.
	tickCh := make(chan struct{}, 10)
	setOnTick(func() {
		select {
		case tickCh <- struct{}{}:
		default:
		}
	})
	defer setOnTick(nil)

	counter := NewUsageTracker()
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("interact:click", 0, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUsageBeaconLoopWithInterval(ctx, counter, 10*time.Millisecond)

	// Wait for 3 tick cycles to complete.
	for i := 0; i < 3; i++ {
		select {
		case <-tickCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tick")
		}
	}

	cancel()

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Fatalf("beacon fired %d times with KABOOM_TELEMETRY=off, want 0", count)
	}
}

func TestUsageBeaconLoop_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	counter := NewUsageTracker()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		startUsageBeaconLoopWithInterval(ctx, counter, 50*time.Millisecond)
		close(done)
	}()

	// Cancel context and verify goroutine exits.
	cancel()

	select {
	case <-done:
		// Good — goroutine exited.
	case <-time.After(2 * time.Second):
		t.Fatal("beacon loop did not stop after context cancel")
	}
}
