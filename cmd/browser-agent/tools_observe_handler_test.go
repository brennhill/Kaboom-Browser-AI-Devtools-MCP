// tools_observe_handler_test.go — Tests top-level observe mode validation.
// Docs: docs/features/feature/observe/index.md
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsObserveDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{bad json`))

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
}

func TestToolsObserveDispatch_MissingWhat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'what' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint contains valid modes
	if !strings.Contains(result.Content[0].Text, "errors") {
		t.Error("hint should list valid modes including 'errors'")
	}
}

func TestToolsObserveDispatch_UnknownMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "nonexistent_mode")
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown mode should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_mode") {
		t.Error("error should mention the invalid mode name")
	}
}

func TestToolsObserveDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, nil)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'what') should return isError:true")
	}
}

// ============================================
// observe(what:"errors") — Response Field Tests
// ============================================

// ============================================
// isServerSideObserveMode Tests
// ============================================

func TestToolsObserve_IsServerSideObserveMode(t *testing.T) {
	t.Parallel()

	serverSide := []string{
		"command_result", "pending_commands", "failed_commands",
		"saved_videos", "recordings", "recording_actions",
		"playback_results", "log_diff_report", "pilot",
	}
	for _, mode := range serverSide {
		if !toolobserve.ServerSideObserveModes[mode] {
			t.Errorf("toolobserve.ServerSideObserveModes[%q] = false, want true", mode)
		}
	}

	clientSide := []string{"errors", "logs", "network_bodies", "actions", "vitals"}
	for _, mode := range clientSide {
		if toolobserve.ServerSideObserveModes[mode] {
			t.Errorf("toolobserve.ServerSideObserveModes[%q] = true, want false", mode)
		}
	}
}

// ============================================
// getValidObserveModes Tests
// ============================================

func TestToolsObserve_GetValidObserveModes(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	modes := strings.Join(h.observeDispatcher.ValidModes(), ", ")
	// Should be sorted
	modeList := strings.Split(modes, ", ")
	for i := 1; i < len(modeList); i++ {
		if modeList[i-1] > modeList[i] {
			t.Errorf("modes not sorted: %q > %q", modeList[i-1], modeList[i])
		}
	}

	// Should contain key modes
	for _, required := range []string{"errors", "logs", "network_bodies", "actions"} {
		if !strings.Contains(modes, required) {
			t.Errorf("valid modes missing %q: %s", required, modes)
		}
	}
}

// ============================================
// Structured Error Field Validation
// ============================================

func TestToolsObserve_StructuredErrorFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "nonexistent")
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected isError:true")
	}

	text := result.Content[0].Text
	errorJSON := extractJSONFromText(text)

	var se mcp.StructuredError
	if err := json.Unmarshal([]byte(errorJSON), &se); err != nil {
		t.Fatalf("structured error JSON parse failed: %v\nraw: %s", err, text)
	}
	if se.ErrorCode == "" {
		t.Error("mcp.StructuredError.ErrorCode should not be empty")
	}
	if se.Message == "" {
		t.Error("mcp.StructuredError.Message should not be empty")
	}
	if se.RecoveryPlaybook == "" {
		t.Error("mcp.StructuredError.RecoveryPlaybook should not be empty")
	}

	// Verify JSON fields are snake_case
	assertSnakeCaseFields(t, errorJSON)
}
