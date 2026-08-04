// tools_interact_page_commands_test.go — Tests interact page-command routing.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
)

// ============================================
// interact(action:"highlight") — Response Fields & Validation
// ============================================

func TestToolsInteractHighlight_MissingSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"highlight"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("highlight without selector should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "selector") {
		t.Error("error should mention 'selector' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHighlight_PilotDisabled(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"highlight","selector":"#main"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("highlight with pilot disabled should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "pilot_disabled") {
		t.Errorf("error code should be 'pilot_disabled', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHighlight_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"highlight","selector":".btn"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("highlight should succeed with pilot enabled, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	// Verify response fields
	for _, field := range []string{"status", "correlation_id", "queued", "final"} {
		if _, ok := data[field]; !ok {
			t.Errorf("highlight response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if corr == "" {
		t.Error("correlation_id should be non-empty")
	}
	if !strings.HasPrefix(corr, "highlight_") {
		t.Errorf("correlation_id should start with 'highlight_', got: %s", corr)
	}

	// Verify pending query created
	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("highlight should create a pending query")
	}
	if pq.Type != "highlight" {
		t.Errorf("pending query type = %q, want 'highlight'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"execute_js") — Response Fields & Validation
// ============================================

func TestToolsInteractExecuteJS_MissingScript(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"execute_js"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("execute_js without script should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "script") {
		t.Error("error should mention missing 'script' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractExecuteJS_InvalidWorld(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"execute_js","script":"1+1","world":"invalid_world"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid world should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("error code should be 'invalid_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "world") {
		t.Error("error should mention 'world' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractExecuteJS_ValidWorlds(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// All valid worlds should pass world validation (fail at pilot check, not world check)
	for _, world := range []string{"auto", "main", "isolated"} {
		t.Run(world, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"execute_js","script":"1+1","world":"`+world+`"}`)
			result := parseToolResult(t, resp)
			// Should fail at pilot check, NOT world validation
			if !result.IsError {
				t.Fatal("should return error (pilot disabled)")
			}
			if strings.Contains(result.Content[0].Text, "world") {
				t.Errorf("world=%q should pass validation, but got world error: %s", world, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractExecuteJS_DefaultWorld(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Omitting world should default to "auto" and pass validation
	resp := callInteractRaw(h, `{"what":"execute_js","script":"1+1"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("should return error (pilot disabled)")
	}
	// Should NOT contain world error
	if strings.Contains(result.Content[0].Text, "world") {
		t.Errorf("default world should pass validation, got: %s", result.Content[0].Text)
	}
}

func TestToolsInteractExecuteJS_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"execute_js","script":"document.title"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("execute_js should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "correlation_id", "queued", "final"} {
		if _, ok := data[field]; !ok {
			t.Errorf("execute_js response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "exec_") {
		t.Errorf("correlation_id should start with 'exec_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"navigate") — Response Fields & Validation
// ============================================

func TestToolsInteractNavigate_MissingURL(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"navigate"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("navigate without url should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "url") {
		t.Error("error should mention 'url' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractNavigate_AssumedEnabledWhenPilotStatusUncertain(t *testing.T) {
	t.Parallel()
	h, server, cap := makeToolHandler(t)
	cap = capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	h = mcpHandler.tools.Executor.(*ToolHandler)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().UpdateTrackedTab(42, "https://example.com", "")

	resp := callInteractRaw(h, `{"what":"navigate","url":"https://example.com"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("navigate should not fail with pilot_disabled during startup uncertainty, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Fatalf("status = %v, want queued", data["status"])
	}
}

func TestToolsInteractNavigate_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"navigate","url":"https://example.com"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("navigate should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "nav_") {
		t.Errorf("correlation_id should start with 'nav_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"refresh/back/forward") — Pilot Check
// ============================================

func TestToolsInteractBrowserActions_PilotRequired(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []struct {
		name string
		args string
	}{
		{"refresh", `{"what":"refresh"}`},
		{"back", `{"what":"back"}`},
		{"forward", `{"what":"forward"}`},
		{"new_tab", `{"what":"new_tab","url":"https://example.com"}`},
	}

	for _, tc := range actions {
		t.Run(tc.name, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s with pilot disabled should return isError:true", tc.name)
			}
			if !strings.Contains(result.Content[0].Text, "pilot_disabled") {
				t.Errorf("%s error code should be 'pilot_disabled', got: %s", tc.name, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractBrowserActions_SuccessWithPilot(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	actions := []struct {
		name   string
		args   string
		prefix string
	}{
		{"refresh", `{"what":"refresh"}`, "refresh_"},
		{"back", `{"what":"back"}`, "back_"},
		{"forward", `{"what":"forward"}`, "forward_"},
		{"new_tab", `{"what":"new_tab","url":"https://example.com"}`, "newtab_"},
	}

	for _, tc := range actions {
		t.Run(tc.name, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("%s should succeed, got: %s", tc.name, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "queued" {
				t.Errorf("status = %v, want 'queued'", data["status"])
			}
			corr, _ := data["correlation_id"].(string)
			if !strings.HasPrefix(corr, tc.prefix) {
				t.Errorf("correlation_id should start with %q, got: %s", tc.prefix, corr)
			}
			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

// ============================================
// interact(action:"subtitle") — Response Fields
// ============================================

func TestToolsInteractSubtitle_MissingText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"subtitle"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("subtitle without text should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "text") {
		t.Error("error should mention 'text' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSubtitle_SetText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"subtitle","text":"Hello world"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("subtitle set should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	if queued, ok := data["queued"].(bool); !ok || !queued {
		t.Errorf("queued = %v, want true", data["queued"])
	}
	if final, ok := data["final"].(bool); !ok || final {
		t.Errorf("final = %v, want false", data["final"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "subtitle_") {
		t.Errorf("correlation_id should start with 'subtitle_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSubtitle_ClearText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"subtitle","text":""}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("subtitle clear should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	if queued, ok := data["queued"].(bool); !ok || !queued {
		t.Errorf("queued = %v, want true", data["queued"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}
