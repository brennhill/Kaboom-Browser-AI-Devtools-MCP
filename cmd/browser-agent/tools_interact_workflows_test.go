// Purpose: Tests for interact multi-step workflow execution.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_workflows_test.go — Tests for high-level workflow primitives.
// Pure function tests (isErrorResponse, responseStatus, workflowResult) live in
// internal/tools/interact/workflow_test.go.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// navigate_and_wait_for validation tests
// ============================================

func TestNavigateAndWaitFor_MissingURL(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"wait_for": ".content",
	})
	resp := h.interactAction().HandleNavigateAndWaitFor(req, args)
	assertIsError(t, resp, "url")
}

func TestNavigateAndWaitFor_MissingWaitFor(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"url": "https://example.com",
	})
	resp := h.interactAction().HandleNavigateAndWaitFor(req, args)
	assertIsError(t, resp, "wait_for")
}

func TestNavigateAndWaitFor_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.interactAction().HandleNavigateAndWaitFor(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

// ============================================
// fill_form_and_submit validation tests
// ============================================

func TestFillFormAndSubmit_EmptyFields(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields":          []any{},
		"submit_selector": "button[type=submit]",
	})
	resp := h.interactAction().HandleFillFormAndSubmit(req, args)
	assertIsError(t, resp, "fields")
}

func TestFillFormAndSubmit_MissingSubmit(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields": []map[string]string{
			{"selector": "#email", "value": "test@example.com"},
		},
	})
	resp := h.interactAction().HandleFillFormAndSubmit(req, args)
	assertIsError(t, resp, "submit_selector")
}

func TestFillFormAndSubmit_FieldMissingSelectorAndIndex(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields": []map[string]string{
			{"value": "test@example.com"},
		},
		"submit_selector": "button",
	})
	resp := h.interactAction().HandleFillFormAndSubmit(req, args)
	assertIsError(t, resp, "selector")
}

func TestFillFormAndSubmit_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.interactAction().HandleFillFormAndSubmit(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

// ============================================
// fill_form validation tests
// ============================================

func TestFillForm_EmptyFields(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields": []any{},
	})
	resp := h.interactAction().HandleFillForm(req, args)
	assertIsError(t, resp, "fields")
}

func TestFillForm_MissingFieldSelectorAndIndex(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields": []map[string]string{
			{"value": "test@example.com"},
		},
	})
	resp := h.interactAction().HandleFillForm(req, args)
	assertIsError(t, resp, "selector")
}

func TestFillForm_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.interactAction().HandleFillForm(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

func TestFillForm_NoSubmitRequired(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	// Valid fields with selector — should not error about submit_selector
	args, _ := json.Marshal(map[string]any{
		"fields": []map[string]string{
			{"selector": "#email", "value": "test@example.com"},
		},
	})
	resp := h.interactAction().HandleFillForm(req, args)
	// Should not return a "submit_selector" error (no submit needed for fill_form)
	raw, _ := json.Marshal(resp)
	rawStr := string(raw)
	if contains(rawStr, "submit_selector") {
		t.Error("fill_form should not require submit_selector")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}

// ============================================
// isNotTypeableError unit tests
// ============================================

func TestIsNotTypeableError_TrueForNotTypeable(t *testing.T) {
	t.Parallel()
	// Simulate a response like the extension returns for <select> elements
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result: mcp.JSONErrorResponse("FAILED", map[string]any{
			"status": "complete",
			"result": map[string]any{
				"success": false,
				"error":   "not_typeable",
				"message": "Element is not an input, textarea, or contenteditable: SELECT",
			},
		}),
	}
	if !toolinteract.IsNotTypeableError(resp) {
		t.Error("expected isNotTypeableError to return true for not_typeable error")
	}
}

func TestIsNotTypeableError_FalseForOtherErrors(t *testing.T) {
	t.Parallel()
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result: mcp.JSONErrorResponse("FAILED", map[string]any{
			"status": "complete",
			"result": map[string]any{
				"success": false,
				"error":   "element_not_found",
				"message": "No element found",
			},
		}),
	}
	if toolinteract.IsNotTypeableError(resp) {
		t.Error("expected isNotTypeableError to return false for non-not_typeable errors")
	}
}

func TestIsNotTypeableError_FalseForSuccess(t *testing.T) {
	t.Parallel()
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result: mcp.JSONResponse("OK", map[string]any{
			"status": "complete",
			"result": map[string]any{"success": true},
		}),
	}
	if toolinteract.IsNotTypeableError(resp) {
		t.Error("expected isNotTypeableError to return false for success response")
	}
}

func TestIsNotTypeableError_FalseForJSONRPCError(t *testing.T) {
	t.Parallel()
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Error:   &mcp.JSONRPCError{Code: -32600, Message: "not_typeable in error"},
	}
	if toolinteract.IsNotTypeableError(resp) {
		t.Error("expected isNotTypeableError to return false for JSON-RPC errors")
	}
}

// ============================================
// run_a11y_and_export_sarif tests
// ============================================

func TestRunA11yAndExportSARIF_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.interactAction().HandleRunA11yAndExportSARIF(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

// ============================================
// run_a11y_and_export_sarif tests
// ============================================

func TestRunA11yAndExportSARIF_ValidParams(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"scope": "page",
	})
	// This will fail due to no extension connected, but should not panic
	// and should return a structured error/workflow result
	resp := h.interactAction().HandleRunA11yAndExportSARIF(req, args)
	if resp.JSONRPC != "2.0" {
		t.Error("expected valid JSON-RPC response")
	}
	// The workflow should have a trace in the result
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
}

func TestRunA11yAndExportSARIF_ReusesAnalyzePayload(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	cap.UpdateExtensionStatus(capture.ExtensionStatus{
		TrackingEnabled: true,
		TrackedTabID:    42,
		TrackedTabURL:   "https://example.com",
	})

	syncReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	syncReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), syncReq)

	var a11yQueryCount int32
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })

	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()

		result := json.RawMessage(`{
			"violations": [{
				"id": "color-contrast",
				"impact": "serious",
				"description": "contrast issue",
				"help": "fix contrast",
				"helpUrl": "https://example.com/rule",
				"nodes": [{"target": ["body"]}]
			}],
			"passes": [],
			"incomplete": [],
			"inapplicable": []
		}`)

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				for _, q := range cap.GetPendingQueries() {
					if q.Type != "a11y" {
						continue
					}
					atomic.AddInt32(&a11yQueryCount, 1)
					cap.SetQueryResult(q.ID, result)
				}
			}
		}
	}()

	args, _ := json.Marshal(map[string]any{
		"scope": "body",
	})

	resp := h.interactAction().HandleRunA11yAndExportSARIF(req, args)
	toolResult := parseToolResult(t, resp)
	if toolResult.IsError {
		t.Fatalf("workflow should succeed, got error: %s", toolResult.Content[0].Text)
	}

	if got := atomic.LoadInt32(&a11yQueryCount); got != 1 {
		t.Fatalf("expected exactly 1 a11y query, got %d", got)
	}
}
