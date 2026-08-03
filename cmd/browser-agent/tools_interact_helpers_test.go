// Purpose: Tests for interact helper functions.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_helpers_test.go — Tests for queueComposableSubtitle and queueStateNavigation.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Test environment
// ============================================

type interactHelpersTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

func newInteractHelpersTestEnv(t *testing.T) *interactHelpersTestEnv {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-interact-helpers.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	server.sessionProjectPath = t.TempDir()
	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false) // explicit default for state/pilot-disabled test branches
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	// Simulate extension connection so tests that enable pilot don't hit the extension gate.
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), httpReq)

	// Simulate tab tracking so tests don't hit the tab tracking gate.
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	return &interactHelpersTestEnv{handler: handler, server: server, capture: cap}
}

// enablePilot activates pilot mode using the test helper.
func (e *interactHelpersTestEnv) enablePilot(t *testing.T) {
	t.Helper()
	e.capture.Extension().SetPilotEnabled(true)
}

// ============================================
// queueComposableSubtitle
// ============================================

func TestQueueComposableSubtitle_QueuesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.pageActions.QueueComposableSubtitle(req, "Test subtitle text")

	// Verify a pending query was created
	queries := env.capture.Queries().GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "subtitle" {
			found = true
			// Verify the params contain the text
			var params map[string]string
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse subtitle params: %v", err)
			}
			if params["text"] != "Test subtitle text" {
				t.Errorf("expected text 'Test subtitle text', got %q", params["text"])
			}
			break
		}
	}
	if !found {
		t.Error("expected a pending query of type 'subtitle' to be created")
	}
}

func TestQueueComposableSubtitle_CorrelationIDHasPrefix(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.pageActions.QueueComposableSubtitle(req, "text")

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "subtitle" {
			if len(q.CorrelationID) < 9 || q.CorrelationID[:9] != "subtitle_" {
				t.Errorf("expected correlation_id to start with 'subtitle_', got %q", q.CorrelationID)
			}
			return
		}
	}
	t.Error("subtitle query not found")
}

func TestQueueComposableSubtitle_EmptyText(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	// Empty text is valid (clears the subtitle)
	env.handler.pageActions.QueueComposableSubtitle(req, "")

	queries := env.capture.Queries().GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "subtitle" {
			found = true
			var params map[string]string
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse params: %v", err)
			}
			if params["text"] != "" {
				t.Errorf("expected empty text, got %q", params["text"])
			}
		}
	}
	if !found {
		t.Error("expected subtitle query even for empty text")
	}
}

func TestQueueComposableSubtitle_UniqueCorrelationIDs(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.pageActions.QueueComposableSubtitle(req, "first")
	env.handler.pageActions.QueueComposableSubtitle(req, "second")

	queries := env.capture.Queries().GetPendingQueries()
	ids := make(map[string]bool)
	for _, q := range queries {
		if q.Type == "subtitle" {
			if ids[q.CorrelationID] {
				t.Errorf("duplicate correlation_id: %q", q.CorrelationID)
			}
			ids[q.CorrelationID] = true
		}
	}
	if len(ids) < 2 {
		t.Errorf("expected at least 2 unique subtitle correlation IDs, got %d", len(ids))
	}
}

// ============================================
// queueComposableActionDiff (#343, #9.R6)
// ============================================

func TestQueueComposableActionDiff_QueuesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.pageActions.QueueComposableActionDiff(req)

	// Verify a pending query was created with type "dom_action"
	queries := env.capture.Queries().GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse action_diff params: %v", err)
			}
			if params["action"] != "action_diff" {
				continue // different dom_action, skip
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a pending query of type 'dom_action' with action 'action_diff'")
	}
}

func TestQueueComposableActionDiff_CorrelationIDPrefix(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.pageActions.QueueComposableActionDiff(req)

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] != "action_diff" {
				continue
			}
			if !strings.HasPrefix(q.CorrelationID, "dom_action_diff_") {
				t.Errorf("expected correlation_id prefix 'dom_action_diff_', got %q", q.CorrelationID)
			}
			return
		}
	}
	t.Error("action_diff query not found")
}

func TestQueueComposableActionDiff_HasTimeoutParam(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.pageActions.QueueComposableActionDiff(req)

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] != "action_diff" {
				continue
			}
			timeoutMs, ok := params["timeout_ms"].(float64)
			if !ok || timeoutMs <= 0 {
				t.Errorf("expected positive timeout_ms, got %v", params["timeout_ms"])
			}
			return
		}
	}
	t.Error("action_diff query not found")
}

// ============================================
// queueStateNavigation
// ============================================

func TestQueueStateNavigation_QueuesBrowserAction(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)

	stateData := map[string]any{
		"url":   "https://example.com/page",
		"title": "Test Page",
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.stateInteractHandler.QueueStateNavigation(req, stateData)

	// Should have queued a browser_action query
	queries := env.capture.Queries().GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "browser_action" {
			found = true
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse nav params: %v", err)
			}
			if params["action"] != "navigate" {
				t.Errorf("expected action 'navigate', got %v", params["action"])
			}
			if params["url"] != "https://example.com/page" {
				t.Errorf("expected url 'https://example.com/page', got %v", params["url"])
			}
			break
		}
	}
	if !found {
		t.Error("expected a pending query of type 'browser_action'")
	}

	// Should have mutated stateData
	if stateData["navigation_queued"] != true {
		t.Error("expected navigation_queued=true in stateData")
	}
	if _, ok := stateData["correlation_id"].(string); !ok {
		t.Error("expected correlation_id string in stateData")
	}
}

func TestQueueStateNavigation_SkipsWhenPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	// Do NOT enable pilot

	stateData := map[string]any{
		"url":   "https://example.com/page",
		"title": "Test Page",
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.stateInteractHandler.QueueStateNavigation(req, stateData)

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when pilot is disabled")
		}
	}

	if _, ok := stateData["navigation_queued"]; ok {
		t.Error("stateData should not have navigation_queued when pilot is disabled")
	}
}

func TestQueueStateNavigation_SkipsWhenURLEmpty(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"url":   "",
		"title": "No URL",
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.stateInteractHandler.QueueStateNavigation(req, stateData)

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when URL is empty")
		}
	}
}

func TestQueueStateNavigation_SkipsWhenURLMissing(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"title": "No URL key",
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.stateInteractHandler.QueueStateNavigation(req, stateData)

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when URL key is missing")
		}
	}
}

func TestQueueStateNavigation_SkipsWhenURLNotString(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"url": 12345, // not a string
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.stateInteractHandler.QueueStateNavigation(req, stateData)

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when URL is not a string")
		}
	}
}

func TestQueueStateNavigation_CorrelationIDHasNavPrefix(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)

	stateData := map[string]any{
		"url": "https://example.com",
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.stateInteractHandler.QueueStateNavigation(req, stateData)

	corrID, ok := stateData["correlation_id"].(string)
	if !ok {
		t.Fatal("expected correlation_id to be set")
	}
	if len(corrID) < 4 || corrID[:4] != "nav_" {
		t.Errorf("expected correlation_id to start with 'nav_', got %q", corrID)
	}
}
