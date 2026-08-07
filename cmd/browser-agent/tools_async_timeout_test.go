// tools_async_timeout_test.go — Tests for configurable timeout_ms in MaybeWaitForCommand.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func parseAsyncResponseData(t *testing.T, rawResult json.RawMessage) map[string]any {
	t.Helper()
	var toolResult mcp.MCPToolResult
	if err := json.Unmarshal(rawResult, &toolResult); err != nil {
		t.Fatalf("decode MCP result: %v", err)
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("MCP result has no content")
	}
	jsonText := extractJSONFromText(toolResult.Content[0].Text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
	return data
}

// ============================================
// Issue #275: Auto-poll async commands with timeout_ms
// ============================================

func TestMaybeWaitForCommand_TimeoutMs_CustomTimeout(t *testing.T) {
	t.Parallel()

	handler, _, cap := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	correlationID := "test-timeout-ms-123"
	cap.Queries().RegisterCommand(correlationID, "q-timeout-ms-123", 60*time.Second)

	// Connect extension (fast path — no long-poll)
	capturefixture.Connect(cap)

	response := make(chan mcp.JSONRPCResponse, 1)
	start := time.Now()
	go func() {
		response <- handler.asyncCommands.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":2000}`), "Queued")
	}()
	cap.Queries().ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"success":true,"data":"custom-timeout"}`), "")
	resp := <-response
	elapsed := time.Since(start)

	result := parseAsyncResponseData(t, resp.Result)
	if result["status"] != "complete" {
		t.Errorf("Expected status=complete with timeout_ms=2000, got %v", result["status"])
	}
	if elapsed > 2*time.Second {
		t.Errorf("Should have completed well within 2s, took %v", elapsed)
	}
}

func TestMaybeWaitForCommand_TimeoutMs_ShortTimeout(t *testing.T) {
	handler, _, cap := makeToolHandler(t)
	handler.asyncCommands.Wait.PollInterval = 50 * time.Millisecond
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	correlationID := "test-short-timeout-123"
	cap.Queries().RegisterCommand(correlationID, "q-short-123", 60*time.Second)

	// Connect extension (fast path — no long-poll)
	capturefixture.Connect(cap)

	// Set a very short timeout_ms — command will not complete in time
	start := time.Now()
	resp := handler.asyncCommands.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":300}`), "Queued")
	elapsed := time.Since(start)

	result := parseAsyncResponseData(t, resp.Result)

	// Should return still_processing, not complete
	status, _ := result["status"].(string)
	if status != "still_processing" {
		t.Errorf("Expected status=still_processing with short timeout, got %v", status)
	}

	// Should have taken roughly the timeout duration, not the full 15s default
	if elapsed > 2*time.Second {
		t.Errorf("Should have timed out in ~300ms, took %v", elapsed)
	}
}

func TestMaybeWaitForCommand_TimeoutMs_ZeroUsesDefault(t *testing.T) {
	t.Parallel()

	handler, _, _ := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-zero-timeout-123"

	// With no extension connected and timeout_ms=0, should use default behavior
	// (which fails fast since extension is not connected)
	resp := handler.asyncCommands.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":0}`), "Queued")

	result := parseAsyncResponseData(t, resp.Result)
	// Without extension connected, should get an error
	if _, hasError := result["error"]; !hasError {
		// The response should be an error about extension not connected
		status, _ := result["status"].(string)
		if status == "complete" {
			t.Error("timeout_ms=0 should not magically complete")
		}
	}
}

func TestMaybeWaitForCommand_SyncFalse_ReturnsCorrelationID(t *testing.T) {
	t.Parallel()

	handler, _, _ := makeToolHandler(t)
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-async-275"

	// sync=false should return queued with correlation_id
	resp := handler.asyncCommands.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"sync":false}`), "Queued")

	result := parseAsyncResponseData(t, resp.Result)
	if result["status"] != "queued" {
		t.Errorf("Expected status=queued with sync=false, got %v", result["status"])
	}
	if result["correlation_id"] != correlationID {
		t.Errorf("Expected correlation_id=%s, got %v", correlationID, result["correlation_id"])
	}
}

func TestMaybeWaitForCommand_TimeoutMs_NegativeIgnored(t *testing.T) {
	t.Parallel()

	handler, _, _ := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-neg-timeout-123"

	// Negative timeout_ms should be treated as default (not infinite)
	// Without extension, should fail fast
	resp := handler.asyncCommands.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":-1}`), "Queued")

	result := parseAsyncResponseData(t, resp.Result)
	// Should not hang — verify we got a response
	if result == nil {
		t.Error("Should have gotten a response even with negative timeout_ms")
	}
}

func TestAnalyze_LinkHealth_SyncTrue_WaitsForResult(t *testing.T) {
	t.Parallel()

	handler, _, cap := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "test-client"}

	// Connect extension (fast path — no long-poll)
	capturefixture.Connect(cap)

	installCompletedCommandWait(t, handler, "link_health_", json.RawMessage(`{"success":true,"healthy":5,"broken":0}`))

	// sync=true (default) should wait for the result
	args := json.RawMessage(`{"what":"link_health","domain":"example.com"}`)
	resp := handler.analyzeDispatcher.Handle(req, args)

	result := parseAsyncResponseData(t, resp.Result)
	status, _ := result["status"].(string)
	// With sync=true (default), it should either complete or still_processing
	// (depending on timing), not "queued"
	if status == "queued" {
		t.Error("sync=true (default) should NOT return queued status")
	}
}

func TestAnalyze_LinkHealth_SyncFalse_ReturnsCorrelationID(t *testing.T) {
	t.Parallel()

	handler, _, _ := makeToolHandler(t)
	handler.coldStartTimeout = 0

	// Don't need extension connected for sync=false
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"what":"link_health","sync":false}`)
	resp := handler.analyzeDispatcher.Handle(req, args)

	result := parseAsyncResponseData(t, resp.Result)
	if result["status"] != "queued" {
		t.Errorf("sync=false should return queued, got %v", result["status"])
	}
	corrID, _ := result["correlation_id"].(string)
	if corrID == "" {
		t.Error("sync=false should return correlation_id")
	}
	if !strings.HasPrefix(corrID, "link_health_") {
		t.Errorf("correlation_id should have link_health_ prefix, got %s", corrID)
	}
}

func TestAnalyze_Dom_TimeoutMs_Respected(t *testing.T) {
	handler, _, cap := makeToolHandler(t)
	// Keep the injected command seam from subdividing the caller's initial
	// budget; it returns immediately and never waits on this duration.
	handler.asyncCommands.Wait.PollInterval = time.Hour
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "test-client"}

	// Connect extension (fast path — no long-poll)
	capturefixture.Connect(cap)

	waits := installCompletedCommandWait(t, handler, "dom_", json.RawMessage(`{"success":true,"elements":[]}`))

	// Set timeout_ms=1000 — should be enough to catch the 200ms result
	args := json.RawMessage(`{"what":"dom","selector":"div","timeout_ms":1000}`)
	resp := handler.analyzeDispatcher.Handle(req, args)
	if len(*waits) != 1 {
		t.Fatalf("wait calls = %d, want exactly one terminal completion", len(*waits))
	}
	if (*waits)[0] < 749*time.Millisecond || (*waits)[0] > 750*time.Millisecond {
		t.Fatalf("initial wait budget = %v, want the 750ms phase from timeout_ms=1000", (*waits)[0])
	}

	result := parseAsyncResponseData(t, resp.Result)
	status, _ := result["status"].(string)
	if status == "queued" {
		t.Error("With sync=true (default) and timeout_ms=1000, should not return queued")
	}
}

// installCompletedCommandWait makes command settlement an explicit test seam.
// The production queue and correlation registration still run; only the
// scheduler-dependent extension response is replaced with a terminal result.
func installCompletedCommandWait(t *testing.T, handler *ToolHandler, prefix string, result json.RawMessage) *[]time.Duration {
	t.Helper()
	waits := make([]time.Duration, 0, 1)
	handler.asyncCommands.Wait.Command = func(correlationID string, timeout time.Duration) (*queries.CommandResult, bool) {
		if !strings.HasPrefix(correlationID, prefix) {
			t.Fatalf("correlation ID %q does not have prefix %q", correlationID, prefix)
		}
		waits = append(waits, timeout)
		command, found := handler.capture.Queries().GetCommandResult(correlationID)
		if !found || command == nil {
			t.Fatalf("command %q was not registered before waiting", correlationID)
		}
		completed := *command
		completed.Status = "complete"
		completed.Result = append(json.RawMessage(nil), result...)
		completed.CompletedAt = completed.CreatedAt
		return &completed, true
	}
	return &waits
}
