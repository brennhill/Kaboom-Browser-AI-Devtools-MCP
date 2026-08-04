// e2e_reporting_test.go — Tests app-telemetry event envelopes and payloads.
// Docs: docs/features/feature/app-telemetry/index.md

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
)

// collectAll drains all beacons from the channel within a deadline.
func collectAll(ch chan map[string]any, deadline time.Duration) []map[string]any {
	var all []map[string]any
	timer := time.After(deadline)
	for {
		select {
		case body := <-ch:
			all = append(all, body)
		case <-timer:
			return all
		}
	}
}

// filterByEvent returns only beacons with the given event type.
func filterByEvent(beacons []map[string]any, event string) []map[string]any {
	var out []map[string]any
	for _, b := range beacons {
		if b["event"] == event {
			out = append(out, b)
		}
	}
	return out
}

func TestE2E_IncidentLifecycleEmitsDistinctBoundedTransitions(t *testing.T) {
	received := captureBeacon(t)
	store := incident.NewStore(4, QueueReliability)
	key, err := store.Detect(incident.Report{Code: incident.CodeStateRecoveryFailed, CorrelationID: "local-only", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !store.Retry(key, 1, 1) || !store.Recover(key, 1) {
		t.Fatal("canonical recovery lifecycle did not advance")
	}

	seen := make(map[string]int)
	for range 3 {
		body := waitForEvent(t, received, "app_error")
		bucket, _ := body["attempt_bucket"].(string)
		outcome, _ := body["outcome"].(string)
		seen[outcome+":"+bucket]++
		for _, forbidden := range []string{"correlation_id", "generation", "detail", "history"} {
			if _, exists := body[forbidden]; exists {
				t.Fatalf("lifecycle telemetry leaked %q: %#v", forbidden, body)
			}
		}
	}
	for _, transition := range []string{"pending:0", "pending:1", "recovered:1"} {
		if seen[transition] != 1 {
			t.Fatalf("transition counts = %#v, want exactly one %s", seen, transition)
		}
	}
}

// requireEnvelope checks all required shared envelope fields are present and valid.
func requireEnvelope(t *testing.T, body map[string]any, label string) {
	t.Helper()
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		val, ok := body[field]
		if !ok {
			t.Errorf("[%s] missing required envelope field: %s", label, field)
			continue
		}
		if _, isStr := val.(string); !isStr {
			t.Errorf("[%s] envelope field %s is %T, want string", label, field, val)
		}
	}
	// iid: 12-char hex
	if iid, ok := body["iid"].(string); ok {
		if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(iid) {
			t.Errorf("[%s] iid = %q, want 12-char hex", label, iid)
		}
	}
	// sid: 16-char hex
	if sid, ok := body["sid"].(string); ok {
		if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(sid) {
			t.Errorf("[%s] sid = %q, want 16-char hex", label, sid)
		}
	}
	// ts: valid RFC3339
	if ts, ok := body["ts"].(string); ok {
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			t.Errorf("[%s] ts = %q, not valid RFC3339: %v", label, ts, err)
		}
	}
}

// ---------- E2E: tool_call event ----------

func TestE2E_ToolCall_SuccessPayload(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	SetLLMName("cursor")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()
	tracker.RecordToolCall("interact:click", 123*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")
	requireEnvelope(t, body, "tool_call/success")

	if body["family"] != "interact" {
		t.Errorf("family = %v, want interact", body["family"])
	}
	if body["name"] != "click" {
		t.Errorf("name = %v, want click", body["name"])
	}
	if body["tool"] != "interact:click" {
		t.Errorf("tool = %v, want interact:click", body["tool"])
	}
	if body["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", body["outcome"])
	}
	if ms, ok := body["latency_ms"].(float64); !ok || ms != 123 {
		t.Errorf("latency_ms = %v, want 123", body["latency_ms"])
	}
	if body["llm"] != "cursor" {
		t.Errorf("llm = %v, want cursor", body["llm"])
	}
	// async_outcome must be absent (not null).
	if val, exists := body["async_outcome"]; exists && val != nil {
		t.Errorf("async_outcome = %v, want absent", val)
	}
}

func TestE2E_ToolCall_ErrorPayload(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("analyze:security", 50*time.Millisecond, true)

	body := waitForEvent(t, received, "tool_call")
	requireEnvelope(t, body, "tool_call/error")

	if body["outcome"] != "error" {
		t.Errorf("outcome = %v, want error", body["outcome"])
	}
	if body["family"] != "analyze" {
		t.Errorf("family = %v, want analyze", body["family"])
	}
	if body["name"] != "security" {
		t.Errorf("name = %v, want security", body["name"])
	}
}

// ---------- E2E: first_tool_call event ----------

func TestE2E_FirstToolCall_FiredOncePerInstall(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	resetInstallIDState()
	resetFirstToolCallState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetFirstToolCallState()
		resetKaboomDir()
	})

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	tracker.RecordToolCall("observe:page", 0, false)
	tracker.RecordToolCall("interact:click", 0, false)

	all := collectAll(received, 3*time.Second)
	firsts := filterByEvent(all, "first_tool_call")

	if len(firsts) != 1 {
		t.Fatalf("first_tool_call fired %d times, want exactly 1", len(firsts))
	}

	body := firsts[0]
	requireEnvelope(t, body, "first_tool_call")
	if body["family"] != "observe" {
		t.Errorf("family = %v, want observe (first tool called)", body["family"])
	}
	if body["tool"] != "observe:page" {
		t.Errorf("tool = %v, want observe:page", body["tool"])
	}
}

// ---------- E2E: session_start event ----------

func TestE2E_AppError_AllCategories(t *testing.T) {
	categories := []struct {
		category incident.Code
		wantKind string
		wantSev  string
		wantSrc  string
	}{
		{incident.CodeDaemonPanic, "internal", "fatal", "daemon"},
		{incident.CodeBridgeConnectionError, "integration", "error", "bridge"},
		{incident.CodeExtensionDisconnect, "integration", "warning", "extension"},
		{incident.CodeInstallConfigError, "internal", "error", "installer"},
	}

	for _, tc := range categories {
		t.Run(string(tc.category), func(t *testing.T) {
			received := captureBeacon(t)
			AppError(tc.category)

			body := waitForEvent(t, received, "app_error")
			requireEnvelope(t, body, "app_error/"+string(tc.category))

			if body["error_kind"] != tc.wantKind {
				t.Errorf("error_kind = %v, want %v", body["error_kind"], tc.wantKind)
			}
			if body["severity"] != tc.wantSev {
				t.Errorf("severity = %v, want %v", body["severity"], tc.wantSev)
			}
			if body["source"] != tc.wantSrc {
				t.Errorf("source = %v, want %v", body["source"], tc.wantSrc)
			}
			// error_code should be UPPER_SNAKE_CASE.
			code, _ := body["error_code"].(string)
			if !regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`).MatchString(code) {
				t.Errorf("error_code = %q, want UPPER_SNAKE_CASE", code)
			}
		})
	}
}

// ---------- E2E: usage_summary event ----------

func TestE2E_EmptyToolKey_NoBlowup(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	// Empty key should not panic or crash — just records as empty.
	tracker.RecordToolCall("", 0, false)

	body := waitForEvent(t, received, "tool_call")
	requireEnvelope(t, body, "tool_call/empty_key")

	// Family and name should be empty strings (splitKey on "" returns "", "").
	if body["family"] != "" {
		t.Errorf("family = %q, want empty", body["family"])
	}
	if body["tool"] != "" {
		t.Errorf("tool = %q, want empty", body["tool"])
	}
}

// ---------- E2E: beacon timeout does not block caller ----------

func TestE2E_SlowServer_DoesNotBlockCaller(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	overrideEndpoint(srv.URL)
	t.Cleanup(resetEndpoint)

	resetSessionState()
	tracker := NewUsageTracker()

	start := time.Now()
	tracker.RecordToolCall("observe:page", 0, false)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("RecordToolCall blocked for %v with slow server — should return immediately", elapsed)
	}
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("telemetry request did not reach the blocked test transport")
	}
	close(releaseRequest)
	waitForBeaconDeliveryIdle()
}

// ---------- E2E: JSON serialization roundtrip ----------

func TestE2E_BeaconJSON_Roundtrip(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	SetLLMName("codex")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()
	tracker.RecordToolCall("generate:test", 75*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")

	// Re-serialize and re-parse to verify clean JSON.
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal beacon: %v", err)
	}
	var roundtrip map[string]any
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal beacon: %v", err)
	}

	// All fields should survive the roundtrip.
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel", "llm", "family", "name", "tool", "outcome", "latency_ms"} {
		if _, ok := roundtrip[field]; !ok {
			t.Errorf("field %q lost in JSON roundtrip", field)
		}
	}
}
