// Purpose: Tests for capture sync behavior in security mode.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
)

func TestHandleSync_IncludesSecurityModeOverridesWhenInsecureModeActive(t *testing.T) {
	t.Parallel()
	cap := NewCapture()
	cap.Extension().SetSecurityMode("insecure_proxy", []string{"csp_headers"})

	reqBody, err := json.Marshal(syncruntime.SyncRequest{
		ExtSessionID: "ext-session-1",
		Settings: &syncruntime.SyncSettings{
			PilotEnabled: true,
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	newSyncHandlerForTest(cap).HandleSync(w, req)
	if w.Code != 200 {
		t.Fatalf("HandleSync status = %d, want 200", w.Code)
	}

	var resp syncruntime.SyncResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp.CaptureOverrides["security_mode"]; got != "insecure_proxy" {
		t.Fatalf("capture_overrides.security_mode = %q, want insecure_proxy", got)
	}
	if got := resp.CaptureOverrides["production_parity"]; got != "false" {
		t.Fatalf("capture_overrides.production_parity = %q, want false", got)
	}
	if got := resp.CaptureOverrides["insecure_rewrites_applied"]; got != "csp_headers" {
		t.Fatalf("capture_overrides.insecure_rewrites_applied = %q, want csp_headers", got)
	}
}

func TestHandleSync_DefaultSecurityModeOverridesEmpty(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	req := httptest.NewRequest("POST", "/sync", bytes.NewReader([]byte(`{"ext_session_id":"ext-default"}`)))
	w := httptest.NewRecorder()
	newSyncHandlerForTest(cap).HandleSync(w, req)
	if w.Code != 200 {
		t.Fatalf("HandleSync status = %d, want 200", w.Code)
	}

	var resp syncruntime.SyncResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.CaptureOverrides) != 0 {
		t.Fatalf("capture_overrides should be empty in normal mode, got: %#v", resp.CaptureOverrides)
	}
}

func FuzzSyncRequestCanonicalRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"ext_session_id":"generation-1"}`),
		[]byte(`{"ext_session_id":"generation-1","command_results":[{"id":"command-1","status":"complete","result":{"ok":true}}]}`),
		[]byte(`{"ext_session_id":"generation-1","in_progress":[{"id":"command-1","status":"running","progress_pct":50}]}`),
		[]byte(`{"ext_session_id":`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		var request syncruntime.SyncRequest
		if json.Unmarshal(raw, &request) != nil {
			return
		}
		encoded, err := json.Marshal(request)
		if err != nil || !json.Valid(encoded) {
			t.Fatalf("canonical sync request could not serialize: %v", err)
		}
		var roundTrip syncruntime.SyncRequest
		if err := json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if request.ExtSessionID != roundTrip.ExtSessionID || len(request.CommandResults) != len(roundTrip.CommandResults) ||
			len(request.InProgress) != len(roundTrip.InProgress) {
			t.Fatal("canonical sync identity or message cardinality changed during round trip")
		}
	})
}
