// tools_interact_handler_test.go — Tests top-level interact dispatch validation.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
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

func TestToolsInteractDispatch_RejectsObserveScreenshotMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"screenshot"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("interact screenshot should return isError:true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "unknown_mode") {
		t.Fatalf("interact screenshot should be rejected as unknown_mode. Got: %s", text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_RejectsStateActionAliases(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	for _, action := range []string{"state_save", "state_load", "state_list", "state_delete"} {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, fmt.Sprintf(`{"what":"%s"}`, action))
			result := parseToolResult(t, resp)
			text := result.Content[0].Text
			if !strings.Contains(text, "unknown_mode") {
				t.Fatalf("%s should be rejected as unknown_mode: %s", action, text)
			}
		})
	}
}

func TestToolsInteractDispatch_RejectsRecordingActionAliases(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	for _, action := range []string{"record_start", "record_stop"} {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, fmt.Sprintf(`{"what":"%s"}`, action))
			result := parseToolResult(t, resp)
			text := result.Content[0].Text
			if !strings.Contains(text, "unknown_mode") {
				t.Fatalf("%s should be rejected as unknown_mode: %s", action, text)
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

func TestToolsInteractDispatch_ActionDoesNotSelectMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"action":"list_states"}`)
	result := parseToolResult(t, resp)
	if !result.IsError || !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Fatalf("action must not act as a mode selector: %s", result.Content[0].Text)
	}
}

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
