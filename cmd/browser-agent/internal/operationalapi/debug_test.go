// debug_test.go — Tests for debug telemetry endpoints.

package operationalapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func stubHandler(counter *telemetry.UsageTracker) *Handler {
	return New(Options{UsageTracker: func() *telemetry.UsageTracker { return counter }})
}

// M5: debugEndpointsEnabled respects KABOOM_DEBUG env var.
func TestDebugEndpointsEnabled_Unset(t *testing.T) {
	t.Setenv("KABOOM_DEBUG", "")
	if DebugEndpointsEnabled() {
		t.Fatal("debugEndpointsEnabled() = true, want false when KABOOM_DEBUG is empty")
	}
}

func TestDebugEndpointsEnabled_Set(t *testing.T) {
	t.Setenv("KABOOM_DEBUG", "1")
	if !DebugEndpointsEnabled() {
		t.Fatal("debugEndpointsEnabled() = false, want true when KABOOM_DEBUG=1")
	}
}

func TestDebugEndpointsEnabled_WrongValue(t *testing.T) {
	t.Setenv("KABOOM_DEBUG", "true")
	if DebugEndpointsEnabled() {
		t.Fatal("debugEndpointsEnabled() = true, want false when KABOOM_DEBUG=true (must be exactly '1')")
	}
}

func TestDebugUsage_GET_EmptyCounter(t *testing.T) {
	t.Parallel()
	handler := stubHandler(telemetry.NewUsageTracker())

	req := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugUsage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts type = %T, want map", body["counts"])
	}
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty", counts)
	}
}

func TestDebugUsage_GET_PopulatedCounter(t *testing.T) {
	t.Parallel()
	counter := telemetry.NewUsageTracker()
	counter.RecordToolCall("observe:page", 0, false)
	counter.RecordToolCall("observe:page", 0, false)
	counter.RecordToolCall("interact:click", 0, false)

	handler := stubHandler(counter)

	req := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugUsage(rr, req)

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	counts := body["counts"].(map[string]any)
	if counts["observe:page"] != float64(2) {
		t.Errorf("observe:page = %v, want 2", counts["observe:page"])
	}
	if counts["interact:click"] != float64(1) {
		t.Errorf("interact:click = %v, want 1", counts["interact:click"])
	}
}

func TestDebugUsage_GET_DoesNotReset(t *testing.T) {
	t.Parallel()
	counter := telemetry.NewUsageTracker()
	counter.RecordToolCall("observe:page", 0, false)

	handler := stubHandler(counter)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeDebugUsage(rr1, req1)

	// Second request — counts should still be there (Peek, not SwapAndReset)
	req2 := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeDebugUsage(rr2, req2)

	var body map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &body)
	counts := body["counts"].(map[string]any)
	if counts["observe:page"] != float64(1) {
		t.Errorf("observe:page = %v after second GET, want 1 (should not reset)", counts["observe:page"])
	}
}

func TestDebugUsage_POST_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := stubHandler(telemetry.NewUsageTracker())

	req := httptest.NewRequest(http.MethodPost, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugUsage(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestDebugUsage_NilCounter(t *testing.T) {
	t.Parallel()
	handler := stubHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/debug/usage", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugUsage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	counts := body["counts"].(map[string]any)
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty for nil counter", counts)
	}
}

func TestDebugBeaconFlush_POST_WithData(t *testing.T) {
	t.Parallel()
	counter := telemetry.NewUsageTracker()
	counter.RecordToolCall("observe:errors", 0, false)
	counter.RecordToolCall("configure:health", 0, false)

	handler := stubHandler(counter)

	req := httptest.NewRequest(http.MethodPost, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugBeaconFlush(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// flushed >= 2 (may include session_depth key).
	if flushed, ok := body["flushed"].(float64); !ok || flushed < 2 {
		t.Errorf("flushed = %v, want >= 2", body["flushed"])
	}

	payload, ok := body["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map", body["payload"])
	}
	if payload["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", payload["event"])
	}
	if _, ok := payload["iid"].(string); !ok {
		t.Error("missing iid in payload")
	}
	if _, ok := payload["sid"].(string); !ok {
		t.Error("missing sid in payload")
	}

	// Verify tool_stats is present with entries.
	stats, ok := payload["tool_stats"].([]any)
	if !ok {
		t.Fatalf("tool_stats type = %T, want []any", payload["tool_stats"])
	}
	if len(stats) == 0 {
		t.Fatal("tool_stats is empty")
	}

	// Counter should be empty after flush.
	peek := counter.Peek()
	if len(peek) != 0 {
		t.Errorf("counter has %d keys after flush, want 0", len(peek))
	}
}

func TestDebugBeaconFlush_POST_Empty(t *testing.T) {
	t.Parallel()
	handler := stubHandler(telemetry.NewUsageTracker())

	req := httptest.NewRequest(http.MethodPost, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugBeaconFlush(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["flushed"] != float64(0) {
		t.Errorf("flushed = %v, want 0", body["flushed"])
	}
	if body["payload"] != nil {
		t.Errorf("payload = %v, want nil for empty counter", body["payload"])
	}
}

func TestDebugBeaconFlush_POST_NilCounter(t *testing.T) {
	t.Parallel()
	handler := stubHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugBeaconFlush(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["flushed"] != float64(0) {
		t.Errorf("flushed = %v, want 0", body["flushed"])
	}
}

func TestDebugBeaconFlush_GET_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := stubHandler(telemetry.NewUsageTracker())

	req := httptest.NewRequest(http.MethodGet, "/debug/beacon-flush", nil)
	rr := httptest.NewRecorder()
	handler.ServeDebugBeaconFlush(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}
