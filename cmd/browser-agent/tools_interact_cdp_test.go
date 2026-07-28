// tools_interact_cdp_test.go — Validate CDP hardware input handlers and parameter validation.
// Why: Prevents silent regressions in hardware_click and CDP escalation paths.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_cdp_test.go — Tests for hardware_click action and click CDP escalation.
package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// hardware_click — Parameter Validation
// ============================================

func TestToolsInteractHardwareClick_MissingX(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"hardware_click","y":100}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("hardware_click without x should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "x") {
		t.Error("error should mention 'x' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHardwareClick_MissingY(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"hardware_click","x":100}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("hardware_click without y should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "y") {
		t.Error("error should mention 'y' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHardwareClick_MissingBoth(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"hardware_click"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("hardware_click without x/y should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHardwareClick_PilotDisabled(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"hardware_click","x":512,"y":384}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("hardware_click with pilot disabled should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "pilot_disabled") {
		t.Errorf("error code should be 'pilot_disabled', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHardwareClick_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	syncReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	syncReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), syncReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"hardware_click","x":512,"y":384}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("hardware_click should succeed with pilot enabled, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "correlation_id", "queued", "final"} {
		if _, ok := data[field]; !ok {
			t.Errorf("hardware_click response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if corr == "" {
		t.Error("correlation_id should be non-empty")
	}
	if !strings.HasPrefix(corr, "cdp_click_") {
		t.Errorf("correlation_id should start with 'cdp_click_', got: %s", corr)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("hardware_click should create a pending query")
	}
	if pq.Type != "cdp_action" {
		t.Errorf("pending query type = %q, want 'cdp_action'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// click with x/y — CDP Escalation
// ============================================

func TestToolsInteractClick_CDPEscalationWithXY(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	syncReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	syncReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), syncReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"click","selector":"#btn","x":100,"y":200}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("click with x/y should succeed (CDP escalation), got: %s", result.Content[0].Text)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("click with x/y should create a pending query")
	}
	if pq.Type != "cdp_action" {
		t.Errorf("pending query type = %q, want 'cdp_action' (CDP escalation)", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractClick_NoCDPEscalationWithoutXY(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	syncReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	syncReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), syncReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"click","selector":"#btn"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("click without x/y should succeed (DOM path), got: %s", result.Content[0].Text)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("click without x/y should create a pending query")
	}
	if pq.Type != "dom_action" {
		t.Errorf("pending query type = %q, want 'dom_action' (no CDP escalation)", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// hardware_click — Valid Actions List
// ============================================

func TestToolsInteractDispatch_HardwareClickInValidActions(t *testing.T) {
	t.Parallel()
	_, _, _ = makeToolHandler(t)

	validActions := getValidInteractActions()
	if !strings.Contains(validActions, "hardware_click") {
		t.Error("hardware_click should appear in valid interact actions list")
	}
}
