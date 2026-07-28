// Purpose: Tests for capture sync behavior in security mode.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleSync_IncludesSecurityModeOverridesWhenInsecureModeActive(t *testing.T) {
	t.Parallel()
	cap := NewCapture()
	cap.Extension().SetSecurityMode("insecure_proxy", []string{"csp_headers"})

	reqBody, err := json.Marshal(SyncRequest{
		ExtSessionID: "ext-session-1",
		Settings: &SyncSettings{
			PilotEnabled: true,
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	NewSyncHandler(cap).HandleSync(w, req)
	if w.Code != 200 {
		t.Fatalf("HandleSync status = %d, want 200", w.Code)
	}

	var resp SyncResponse
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
	NewSyncHandler(cap).HandleSync(w, req)
	if w.Code != 200 {
		t.Fatalf("HandleSync status = %d, want 200", w.Code)
	}

	var resp SyncResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.CaptureOverrides) != 0 {
		t.Fatalf("capture_overrides should be empty in normal mode, got: %#v", resp.CaptureOverrides)
	}
}
