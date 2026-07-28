// tools_configure_handler_test.go — Tests top-level configure dispatch and response shape.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsConfigureDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_MissingAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'action' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid actions
	text := result.Content[0].Text
	for _, action := range []string{"clear", "health", "noise_rule", "store"} {
		if !strings.Contains(text, action) {
			t.Errorf("hint should list valid action %q", action)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_UnknownAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"nonexistent_action"}`)
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

func TestToolsConfigureDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.configureDispatcher.Handle(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'action') should return isError:true")
	}
}

// ============================================
// Canonical configure action registry tests
// ============================================

func TestToolsConfigure_GetValidConfigureActions(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	actionList := h.configureDispatcher.Actions()
	for i := 1; i < len(actionList); i++ {
		if actionList[i-1] > actionList[i] {
			t.Errorf("actions not sorted: %q > %q", actionList[i-1], actionList[i])
		}
	}

	actions := strings.Join(actionList, ", ")
	for _, required := range []string{"clear", "health", "noise_rule", "store", "load", "streaming"} {
		if !strings.Contains(actions, required) {
			t.Errorf("valid actions missing %q: %s", required, actions)
		}
	}
}

// ============================================
// configure(action:"health") — Response Fields
// ============================================

func TestToolsConfigure_AllActions_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []struct {
		action string
		args   string
	}{
		{"health", `{"what":"health"}`},
		{"telemetry", `{"what":"telemetry"}`},
		{"clear", `{"what":"clear"}`},
		{"noise_rule", `{"what":"noise_rule","noise_action":"list"}`},
		{"load", `{"what":"load"}`},
		{"audit_log", `{"what":"audit_log"}`},
		{"diff_sessions", `{"what":"diff_sessions"}`},
		{"test_boundary_start", `{"what":"test_boundary_start","test_id":"test"}`},
		{"test_boundary_end", `{"what":"test_boundary_end","test_id":"test"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("configure(%s) PANICKED: %v", tc.action, r)
				}
			}()

			resp := callConfigureRaw(h, tc.args)
			if resp.Result == nil {
				t.Fatalf("configure(%s) returned nil result", tc.action)
			}

			result := parseToolResult(t, resp)
			if len(result.Content) == 0 {
				t.Errorf("configure(%s) should return at least one content block", tc.action)
			}
			if result.Content[0].Type != "text" {
				t.Errorf("configure(%s) content type = %q, want 'text'", tc.action, result.Content[0].Type)
			}

			if !result.IsError {
				assertSnakeCaseFields(t, string(resp.Result))
			}
		})
	}
}
