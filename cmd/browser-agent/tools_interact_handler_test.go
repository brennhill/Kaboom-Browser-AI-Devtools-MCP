// Purpose: Tests for interact tool handler dispatch.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_handler_test.go — Comprehensive unit tests for interact tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, parameter validation, and error handling.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsInteractDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_MissingAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'action' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid actions
	text := result.Content[0].Text
	for _, action := range []string{"highlight", "navigate", "execute_js", "click"} {
		if !strings.Contains(text, action) {
			t.Errorf("hint should list valid action %q", action)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_UnknownAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"nonexistent_action"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown action should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_action") {
		t.Error("error should mention the invalid action name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_UnknownActionAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"action":"nonexistent_action"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown action alias should return isError:true")
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block on error path, got %d content blocks", len(result.Content))
	}
}

func TestToolsInteractDispatch_ScreenshotAlias(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"screenshot"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("screenshot alias without tracked tab should return isError:true")
	}
	text := result.Content[0].Text
	if strings.Contains(text, "unknown_mode") {
		t.Fatalf("screenshot alias should not return unknown_mode. Got: %s", text)
	}
	if !strings.Contains(text, "no_data") {
		t.Fatalf("screenshot alias should route to screenshot handler (no_data expected in unit test). Got: %s", text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_StateActionAliases(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	for _, action := range []string{"state_save", "state_load", "state_list", "state_delete"} {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, fmt.Sprintf(`{"what":"%s"}`, action))
			result := parseToolResult(t, resp)
			text := result.Content[0].Text
			if strings.Contains(text, "unknown_mode") {
				t.Fatalf("%s should be recognized alias, got unknown_mode: %s", action, text)
			}
		})
	}
}

func TestToolsInteractDispatch_RecordingActionAliases(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	for _, action := range []string{"record_start", "record_stop"} {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, fmt.Sprintf(`{"action":"%s"}`, action))
			result := parseToolResult(t, resp)
			text := result.Content[0].Text
			if strings.Contains(text, "unknown_mode") {
				t.Fatalf("%s should be recognized alias, got unknown_mode: %s", action, text)
			}
		})
	}
}

func TestToolsInteractDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolInteract(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'action') should return isError:true")
	}
}

func TestToolsInteractDispatch_ActionAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"action":"list_states"}`)
	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks in alias response")
	}
	if strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Fatalf("action alias should not route to unknown_mode, got: %s", result.Content[0].Text)
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block, got %d content blocks", len(result.Content))
	}
}

func TestToolsInteractDispatch_ConflictingWhatAndAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"list_states","action":"navigate"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("conflicting what/action should return isError:true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "invalid_param") {
		t.Fatalf("expected invalid_param, got: %s", text)
	}
	if !strings.Contains(text, "Conflicting parameters") {
		t.Fatalf("expected conflict explanation, got: %s", text)
	}
}

// TestToolsInteractDispatch_SameWhatAndActionAllowed verifies that providing
// the same value for both 'what' and the deprecated 'action' alias is not
// treated as a conflict — it is a no-op redundancy the server should tolerate.
func TestToolsInteractDispatch_SameWhatAndActionAllowed(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"list_states","action":"list_states"}`)
	result := parseToolResult(t, resp)
	// Should not return an invalid_param conflict error — same value is not a conflict.
	if result.IsError && strings.Contains(result.Content[0].Text, "Conflicting parameters") {
		t.Fatalf("same what+action value should not trigger conflict error, got: %s", result.Content[0].Text)
	}
}

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
	cap.SetPilotEnabled(true)
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
	pq := cap.GetLastPendingQuery()
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
	cap.SetPilotEnabled(true)
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
	h, _, cap := makeToolHandler(t)
	cap.SetPilotUnknownForTest()
	mockConnectedTrackedTab(t, cap)

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
	cap.SetPilotEnabled(true)
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
	cap.SetPilotEnabled(true)
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

// ============================================
// interact DOM primitives — Parameter Validation
// ============================================

func TestToolsInteractDOMPrimitives_MissingSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []string{"click", "type", "select", "check", "get_text", "get_value",
		"get_attribute", "set_attribute", "focus", "scroll_to", "wait_for"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"`+action+`"}`)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without selector should return isError:true", action)
			}
			if !strings.Contains(result.Content[0].Text, "selector") {
				t.Errorf("%s error should mention 'selector', got: %s", action, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_IntentActions_NoSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []string{
		"open_composer",
		"submit_active_composer",
		"confirm_top_dialog",
		"dismiss_top_overlay",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"`+action+`"}`)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without selector should still error while pilot is disabled", action)
			}
			if strings.Contains(strings.ToLower(result.Content[0].Text), "selector") {
				t.Errorf("%s should not fail with selector-missing guidance: %s", action, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_ActionSpecificParams(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Actions that require specific params beyond selector
	cases := []struct {
		action  string
		args    string
		missing string
	}{
		{"type", `{"what":"type","selector":"input"}`, "text"},
		{"select", `{"what":"select","selector":"select"}`, "value"},
		{"get_attribute", `{"what":"get_attribute","selector":"div"}`, "name"},
		{"set_attribute", `{"what":"set_attribute","selector":"div"}`, "name"},
	}

	for _, tc := range cases {
		t.Run(tc.action+"_missing_"+tc.missing, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without %s should return isError:true", tc.action, tc.missing)
			}
			if !strings.Contains(result.Content[0].Text, tc.missing) {
				t.Errorf("error should mention missing %q param, got: %s", tc.missing, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_SuccessWithPilot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"input","text":"hello"}`},
		{"select", `{"what":"select","selector":"select","value":"opt1"}`},
		{"check", `{"what":"check","selector":"input[type=checkbox]"}`},
		{"get_text", `{"what":"get_text","selector":"div"}`},
		{"get_value", `{"what":"get_value","selector":"input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"a","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"div","name":"data-test","value":"1"}`},
		{"focus", `{"what":"focus","selector":"input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#footer"}`},
		{"wait_for", `{"what":"wait_for","selector":"#spinner"}`},
		{"key_press", `{"what":"key_press","selector":"input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			h, _, cap := makeToolHandler(t)
			cap.SetPilotEnabled(true)
			mockConnectedTrackedTab(t, cap)

			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("%s should succeed with pilot enabled, got: %s", tc.action, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "queued" {
				t.Errorf("status = %v, want 'queued'", data["status"])
			}
			corr, _ := data["correlation_id"].(string)
			if !strings.HasPrefix(corr, "dom_") {
				t.Errorf("correlation_id should start with 'dom_', got: %s", corr)
			}

			pq := cap.GetLastPendingQuery()
			if pq == nil {
				t.Fatalf("expected pending query for %s", tc.action)
			}
			if !strings.Contains(string(pq.Params), `"action":"`+tc.action+`"`) {
				t.Errorf("pending query params should include canonical action=%q, got: %s", tc.action, string(pq.Params))
			}
			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

func TestToolsInteractDOMPrimitive_AnnotationRectAliasMapsToScopeRect(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"click","selector":".compose","annotation_rect":{"x":120,"y":240,"width":300,"height":180}}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("click with annotation_rect should queue, got error: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query for click")
	}

	var payload map[string]any
	if err := json.Unmarshal(pq.Params, &payload); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	if got, _ := payload["action"].(string); got != "click" {
		t.Fatalf("pending action = %v, want click", payload["action"])
	}
	scopeRect, ok := payload["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("scope_rect missing from normalized payload: %s", string(pq.Params))
	}
	for _, k := range []string{"x", "y", "width", "height"} {
		if _, exists := scopeRect[k]; !exists {
			t.Fatalf("scope_rect missing key %q in payload: %s", k, string(pq.Params))
		}
	}
}

// ============================================
// interact(action:"list_interactive") — near_x/near_y/near_radius → scope_rect conversion (#448)
// ============================================

func TestToolsInteractDOMPrimitive_NearParamsConvertToScopeRect(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"list_interactive","near_x":500,"near_y":300,"near_radius":150}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_interactive with near params should queue, got error: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query for list_interactive")
	}

	var payload map[string]any
	if err := json.Unmarshal(pq.Params, &payload); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	scopeRect, ok := payload["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("scope_rect missing from payload — near_x/near_y/near_radius should convert to scope_rect: %s", string(pq.Params))
	}
	// near_x=500, near_y=300, near_radius=150 → scope_rect={x:350, y:150, width:300, height:300}
	if x, _ := scopeRect["x"].(float64); x != 350 {
		t.Errorf("scope_rect.x = %v, want 350", x)
	}
	if y, _ := scopeRect["y"].(float64); y != 150 {
		t.Errorf("scope_rect.y = %v, want 150", y)
	}
	if w, _ := scopeRect["width"].(float64); w != 300 {
		t.Errorf("scope_rect.width = %v, want 300", w)
	}
	if h, _ := scopeRect["height"].(float64); h != 300 {
		t.Errorf("scope_rect.height = %v, want 300", h)
	}
}

func TestToolsInteractDOMPrimitive_NearParamsDoNotOverrideScopeRect(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	// Explicit scope_rect takes precedence over near params
	resp := callInteractRaw(h, `{"what":"list_interactive","near_x":500,"near_y":300,"near_radius":150,"scope_rect":{"x":0,"y":0,"width":100,"height":100}}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query")
	}

	var payload map[string]any
	if err := json.Unmarshal(pq.Params, &payload); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	scopeRect := payload["scope_rect"].(map[string]any)
	if x, _ := scopeRect["x"].(float64); x != 0 {
		t.Errorf("explicit scope_rect.x should be preserved, got %v", x)
	}
}

// ============================================
// interact(action:"save_state") — Response Fields
// ============================================

func TestToolsInteractSaveState_MissingSnapshotName(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"save_state"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("save_state without snapshot_name should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "snapshot_name") {
		t.Error("error should mention 'snapshot_name' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSaveState_Success(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"save_state","snapshot_name":"test_save"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("save_state should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "snapshot_name", "state"} {
		if _, ok := data[field]; !ok {
			t.Errorf("save_state response missing field %q", field)
		}
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want 'saved'", data["status"])
	}
	if data["snapshot_name"] != "test_save" {
		t.Errorf("snapshot_name = %v, want 'test_save'", data["snapshot_name"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"list_states") — Response Fields
// ============================================

func TestToolsInteractListStates_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"list_states"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_states should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"states", "count"} {
		if _, ok := data[field]; !ok {
			t.Errorf("list_states response missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"list_interactive") — Response Fields
// ============================================

func TestToolsInteractListInteractive_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"list_interactive"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_interactive should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_list_") {
		t.Errorf("correlation_id should start with 'dom_list_', got: %s", corr)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query for list_interactive")
	}
	if !strings.Contains(string(pq.Params), `"action":"list_interactive"`) {
		t.Errorf("pending query params should include canonical action=list_interactive, got: %s", string(pq.Params))
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"get_text", structured:true) — Regression (#390)
// ============================================

func TestToolsInteractGetText_StructuredPassthrough(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"get_text","selector":".accordion","structured":true}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("get_text should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_get_text_") {
		t.Errorf("correlation_id should start with 'dom_get_text_', got: %s", corr)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query for get_text")
	}
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("pending query params should be valid JSON: %v", err)
	}
	if got, _ := params["action"].(string); got != "get_text" {
		t.Fatalf("pending query action = %#v, want get_text", params["action"])
	}
	if got, _ := params["structured"].(bool); !got {
		t.Fatalf("pending query should include structured=true, got: %#v", params["structured"])
	}
}

// ============================================
// validateDOMActionParams Tests
// ============================================

func TestToolsValidateDOMActionParams(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	// Actions without special required params should pass
	for _, action := range []string{"click", "check", "focus", "scroll_to", "wait_for", "key_press"} {
		_, failed := toolinteract.ValidateDOMActionParams(req, action, "", "", "")
		if failed {
			t.Errorf("toolinteract.ValidateDOMActionParams(%q) should not fail for actions without required params", action)
		}
	}

	// "type" requires "text"
	_, failed := toolinteract.ValidateDOMActionParams(req, "type", "", "", "")
	if !failed {
		t.Error("type without text should fail validation")
	}
	_, failed = toolinteract.ValidateDOMActionParams(req, "type", "hello", "", "")
	if failed {
		t.Error("type with text should pass validation")
	}

	// "select" requires "value"
	_, failed = toolinteract.ValidateDOMActionParams(req, "select", "", "", "")
	if !failed {
		t.Error("select without value should fail validation")
	}
	_, failed = toolinteract.ValidateDOMActionParams(req, "select", "", "opt1", "")
	if failed {
		t.Error("select with value should pass validation")
	}

	// "get_attribute" requires "name"
	_, failed = toolinteract.ValidateDOMActionParams(req, "get_attribute", "", "", "")
	if !failed {
		t.Error("get_attribute without name should fail validation")
	}
	_, failed = toolinteract.ValidateDOMActionParams(req, "get_attribute", "", "", "href")
	if failed {
		t.Error("get_attribute with name should pass validation")
	}
}

// truncateToLen pure function tests live in internal/tools/interact/selector_test.go.
