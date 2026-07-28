// Purpose: Tests for synchronous tool call execution.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func newSyncTestHandler(cap *capture.Capture) *ToolHandler {
	return &ToolHandler{
		capture: cap,
		Guards:  toolguard.New(cap, context.Background(), time.Millisecond),
	}
}

func TestMaybeWaitForCommand_SyncByDefault(t *testing.T) {
	// Setup
	cap := capture.NewCapture()
	// coldStartTimeout=0 disables cold-start gate; extension is pre-connected via HandleSync below.
	handler := newSyncTestHandler(cap)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	correlationID := "test-sync-123"
	cap.Queries().RegisterCommand(correlationID, "q-sync-123", 15*time.Second)

	// Mock extension connection manually to avoid nil pointer in HandleSync
	// (Internal knowledge: IsExtensionConnected checks lastSyncSeen)
	// We'll use the proper way to simulate connection: a Sync request.

	// Create a result after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cap.Queries().ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"success":true,"message":"instant result"}`), "")
	}()

	// Simulate extension connection via a Sync call so IsExtensionConnected() returns true
	reqBody := `{"ext_session_id":"test"}`
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(reqBody))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)

	// Call with no explicit sync param (should default to true)
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{}`), "Queued")

	// Verify
	result := parseMCPResponseData(t, resp.Result)

	if result["correlation_id"] != correlationID {
		t.Errorf("Expected correlation_id %s, got %v", correlationID, result["correlation_id"])
	}
	if result["status"] != "complete" {
		t.Errorf("Expected status complete (sync), got %v", result["status"])
	}
}

func TestMaybeWaitForCommand_BackgroundOverride(t *testing.T) {
	cap := capture.NewCapture()
	handler := newSyncTestHandler(cap)
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-bg-123"

	// Call with background: true
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"background":true}`), "Queued")

	result := parseMCPResponseData(t, resp.Result)

	if result["status"] != "queued" {
		t.Errorf("Expected status queued (background), got %v", result["status"])
	}
	if result["lifecycle_status"] != "queued" {
		t.Errorf("Expected lifecycle_status queued (background), got %v", result["lifecycle_status"])
	}
	if result["trace_id"] != correlationID {
		t.Errorf("Expected trace_id %s, got %v", correlationID, result["trace_id"])
	}
}

func TestToolObserveCommandResult_IncludesTraceTimeline(t *testing.T) {
	handler, _, cap := makeToolHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	correlationID := "test-trace-obs-123"

	queryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"what":"click","selector":"button"}`),
		CorrelationID: correlationID,
	}, 30*time.Second, "test-client")
	if queryID == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty query ID")
	}

	_ = cap.Queries().GetPendingQueries() // marks "sent"
	cap.Queries().AcknowledgePendingQuery(queryID)
	cap.Queries().ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"success":true}`), "")

	resp := handler.observeDispatcher.CommandResult(req, json.RawMessage(`{"correlation_id":"test-trace-obs-123"}`))
	data := parseMCPResponseData(t, resp.Result)

	trace, ok := data["trace"].(map[string]any)
	if !ok {
		t.Fatalf("trace field missing or invalid: %v", data["trace"])
	}
	if trace["trace_id"] != correlationID {
		t.Fatalf("trace.trace_id = %v, want %s", trace["trace_id"], correlationID)
	}
	if trace["query_id"] != queryID {
		t.Fatalf("trace.query_id = %v, want %s", trace["query_id"], queryID)
	}

	timeline, _ := trace["timeline"].(string)
	if timeline == "" {
		t.Fatal("trace.timeline should not be empty")
	}
	for _, stage := range []string{"queued", "sent", "started", "resolved"} {
		if !strings.Contains(timeline, stage) {
			t.Fatalf("trace.timeline=%q missing stage %q", timeline, stage)
		}
	}

	if trace["last_stage"] != "resolved" {
		t.Fatalf("trace.last_stage = %v, want resolved", trace["last_stage"])
	}
	if _, ok := trace["events"]; ok {
		t.Fatalf("trace.events should be omitted in summary response, got %v", trace["events"])
	}
}

func TestMaybeWaitForCommand_TimeoutGracefulFallback(t *testing.T) {
	cap := capture.NewCapture()
	handler := newSyncTestHandler(cap)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-timeout-123"

	// Note: We'd need to mock the 15s timeout to be shorter for testing,
	// or just verify it returns "pending" if we don't complete it.
	// For this unit test, we'll assume the 15s wait happens.

	// Start a goroutine to check if it's blocking
	start := time.Now()
	_ = handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"sync":true}`), "Queued")
	duration := time.Since(start)

	// Since we didn't mock connection or result, it should fail fast or timeout.
	// With coldStartTimeout=0, it fails fast if extension not connected.
	if duration > 1*time.Second {
		t.Errorf("Should have failed fast since extension is not connected, took %v", duration)
	}
}

func TestMaybeWaitForCommand_PendingDisconnectReturnsTerminalError(t *testing.T) {
	restoreThreshold := capture.SetExtensionDisconnectThresholdForTesting(200 * time.Millisecond)
	defer restoreThreshold()

	prevInitial := asyncInitialWait
	prevRetry := asyncRetryWait
	prevPoll := asyncPollInterval
	asyncInitialWait = 500 * time.Millisecond
	asyncRetryWait = 200 * time.Millisecond
	asyncPollInterval = 20 * time.Millisecond
	defer func() {
		asyncInitialWait = prevInitial
		asyncRetryWait = prevRetry
		asyncPollInterval = prevPoll
	}()

	cap := capture.NewCapture()
	handler := newSyncTestHandler(cap)
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-disconnect-midwait-123"
	cap.Queries().RegisterCommand(correlationID, "q-disconnect-midwait-123", 5*time.Second)
	cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:   "browser_action",
		Params: json.RawMessage(`{"action":"noop"}`),
	}, 5*time.Second, "test-client")

	// Seed a recent /sync so the first connectivity check passes.
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)

	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"sync":true}`), "Queued")
	result := parseMCPResponseData(t, resp.Result)

	if result["status"] != "error" {
		t.Fatalf("Expected status=error when extension disconnects mid-wait, got %v", result["status"])
	}
	if result["error"] != "extension_disconnected" {
		t.Fatalf("Expected error=extension_disconnected, got %v", result["error"])
	}
	if result["final"] != true {
		t.Fatalf("Expected final=true, got %v", result["final"])
	}
}

// parseMCPResponseData extracts and parses the JSON data from an MCP tool response.
// MCPToolResult wraps data as Content[0].Text = "summary\n{json...}".
func parseMCPResponseData(t *testing.T, rawResult json.RawMessage) map[string]any {
	t.Helper()
	var toolResult mcp.MCPToolResult
	if err := json.Unmarshal(rawResult, &toolResult); err != nil {
		t.Fatalf("Failed to unmarshal MCPToolResult: %v", err)
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("MCPToolResult has no content blocks")
	}
	jsonText := extractJSONFromText(toolResult.Content[0].Text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse JSON from MCP text: %v\nRaw text: %s", err, toolResult.Content[0].Text)
	}
	return data
}

func TestFormatCommandResult_FinalField(t *testing.T) {
	cap := capture.NewCapture()
	handler := newSyncTestHandler(cap)
	req := mcp.JSONRPCRequest{ID: 1}
	now := time.Now()

	t.Run("complete has final true", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-complete",
			Status:        "complete",
			Result:        json.RawMessage(`{"success":true}`),
			CreatedAt:     now,
			CompletedAt:   now.Add(100 * time.Millisecond),
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		data := parseMCPResponseData(t, resp.Result)
		if final, ok := data["final"]; !ok || final != true {
			t.Errorf("Expected final=true for complete, got %v", data["final"])
		}
		if data["lifecycle_status"] != "complete" {
			t.Errorf("Expected lifecycle_status=complete, got %v", data["lifecycle_status"])
		}
	})

	t.Run("error has final true", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-error",
			Status:        "error",
			Error:         "something failed",
			CreatedAt:     now,
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		data := parseMCPResponseData(t, resp.Result)
		if final, ok := data["final"]; !ok || final != true {
			t.Errorf("Expected final=true for error, got %v", data["final"])
		}
	})

	t.Run("pending has final false", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-pending",
			Status:        "pending",
			CreatedAt:     now,
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		data := parseMCPResponseData(t, resp.Result)
		if final, ok := data["final"]; !ok || final != false {
			t.Errorf("Expected final=false for pending, got %v", data["final"])
		}
		if data["lifecycle_status"] != "running" {
			t.Errorf("Expected lifecycle_status=running for pending, got %v", data["lifecycle_status"])
		}
	})

	t.Run("expired returns isError", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-expired",
			Status:        "expired",
			Error:         "timed out",
			CreatedAt:     now,
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		var toolResult mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		if !toolResult.IsError {
			t.Error("Expected isError=true for expired status")
		}
	})

	t.Run("timeout returns isError", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-timeout",
			Status:        "timeout",
			Error:         "no response",
			CreatedAt:     now,
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		var toolResult mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		if !toolResult.IsError {
			t.Error("Expected isError=true for timeout status")
		}
	})
}

func TestFormatCommandResult_ElapsedMs(t *testing.T) {
	cap := capture.NewCapture()
	handler := newSyncTestHandler(cap)
	req := mcp.JSONRPCRequest{ID: 1}
	now := time.Now()

	t.Run("complete has elapsed_ms", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-elapsed",
			Status:        "complete",
			Result:        json.RawMessage(`{"ok":true}`),
			CreatedAt:     now.Add(-500 * time.Millisecond),
			CompletedAt:   now,
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		data := parseMCPResponseData(t, resp.Result)

		elapsedMs, ok := data["elapsed_ms"].(float64)
		if !ok {
			t.Fatal("elapsed_ms missing or not a number")
		}
		if elapsedMs < 400 || elapsedMs > 600 {
			t.Errorf("elapsed_ms = %v, want ~500", elapsedMs)
		}
	})

	t.Run("pending has elapsed_ms", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-pending-elapsed",
			Status:        "pending",
			CreatedAt:     now.Add(-200 * time.Millisecond),
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		data := parseMCPResponseData(t, resp.Result)

		elapsedMs, ok := data["elapsed_ms"].(float64)
		if !ok {
			t.Fatal("elapsed_ms missing or not a number")
		}
		if elapsedMs < 100 {
			t.Errorf("elapsed_ms = %v, want >= 100 (pending uses time.Now())", elapsedMs)
		}
	})

	t.Run("expired has elapsed_ms", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "cmd-expired-elapsed",
			Status:        "expired",
			Error:         "timed out",
			CreatedAt:     now.Add(-1 * time.Second),
		}
		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)
		data := parseMCPResponseData(t, resp.Result)

		elapsedMs, ok := data["elapsed_ms"].(float64)
		if !ok {
			t.Fatal("elapsed_ms missing or not a number")
		}
		if elapsedMs < 900 {
			t.Errorf("elapsed_ms = %v, want >= 900", elapsedMs)
		}
	})
}

func TestFormatCommandResult_ListInteractivePayloadHardening(t *testing.T) {
	cap := capture.NewCapture()
	handler := newSyncTestHandler(cap)
	req := mcp.JSONRPCRequest{ID: 1}
	now := time.Now()

	t.Run("null payload is surfaced as explicit error", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "dom_list_123",
			Status:        "complete",
			Result:        json.RawMessage(`{"value":null,"resolved_tab_id":42}`),
			CreatedAt:     now,
			CompletedAt:   now.Add(100 * time.Millisecond),
		}

		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)

		var toolResult mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
			t.Fatalf("json.Unmarshal(resp.Result) error = %v", err)
		}
		if !toolResult.IsError {
			t.Fatal("expected isError=true when dom_list payload is null")
		}

		data := parseMCPResponseData(t, resp.Result)
		if got, _ := data["error"].(string); got != "list_interactive_missing_payload" {
			t.Fatalf("error = %q, want list_interactive_missing_payload", got)
		}
		resultMap, _ := data["result"].(map[string]any)
		elements, ok := resultMap["elements"].([]any)
		if !ok {
			t.Fatalf("result.elements missing or wrong type: %#v", resultMap["elements"])
		}
		if len(elements) != 0 {
			t.Fatalf("result.elements length = %d, want 0", len(elements))
		}
	})

	t.Run("nil elements are normalized to empty array", func(t *testing.T) {
		cmd := queries.CommandResult{
			CorrelationID: "dom_list_456",
			Status:        "complete",
			Result:        json.RawMessage(`{"success":true,"elements":null}`),
			CreatedAt:     now,
			CompletedAt:   now.Add(100 * time.Millisecond),
		}

		resp := handler.formatCommandResult(req, cmd, cmd.CorrelationID)

		var toolResult mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
			t.Fatalf("json.Unmarshal(resp.Result) error = %v", err)
		}
		if toolResult.IsError {
			t.Fatalf("expected isError=false after normalization, got true: %+v", toolResult)
		}

		data := parseMCPResponseData(t, resp.Result)
		resultMap, _ := data["result"].(map[string]any)
		elements, ok := resultMap["elements"].([]any)
		if !ok {
			t.Fatalf("result.elements missing or wrong type: %#v", resultMap["elements"])
		}
		if len(elements) != 0 {
			t.Fatalf("result.elements length = %d, want 0", len(elements))
		}
	})
}

func TestQueuePosition_And_QueueDepth(t *testing.T) {
	cap := capture.NewCapture()

	// Queue 3 commands
	cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "pos-0",
	})
	cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "pos-1",
	})
	cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "pos-2",
	})

	if depth := cap.Queries().QueueDepth(); depth != 3 {
		t.Errorf("QueueDepth = %d, want 3", depth)
	}

	if pos := cap.Queries().QueuePosition("pos-0"); pos != 0 {
		t.Errorf("QueuePosition(pos-0) = %d, want 0", pos)
	}
	if pos := cap.Queries().QueuePosition("pos-1"); pos != 1 {
		t.Errorf("QueuePosition(pos-1) = %d, want 1", pos)
	}
	if pos := cap.Queries().QueuePosition("pos-2"); pos != 2 {
		t.Errorf("QueuePosition(pos-2) = %d, want 2", pos)
	}
	if pos := cap.Queries().QueuePosition("nonexistent"); pos != -1 {
		t.Errorf("QueuePosition(nonexistent) = %d, want -1", pos)
	}
}
