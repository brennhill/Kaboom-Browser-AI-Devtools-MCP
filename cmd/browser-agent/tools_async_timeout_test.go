// tools_async_timeout_test.go — Tests for configurable timeout_ms in MaybeWaitForCommand.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Issue #275: Auto-poll async commands with timeout_ms
// ============================================

func TestMaybeWaitForCommand_TimeoutMs_CustomTimeout(t *testing.T) {
	t.Parallel()

	handler, _, cap := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	correlationID := "test-timeout-ms-123"
	cap.RegisterCommand(correlationID, "q-timeout-ms-123", 60*time.Second)

	// Connect extension (fast path — no long-poll)
	cap.SimulateExtensionConnectForTest()

	// Complete the command after 200ms
	go func() {
		time.Sleep(200 * time.Millisecond)
		cap.CompleteCommand(correlationID, json.RawMessage(`{"success":true,"data":"custom-timeout"}`), "")
	}()

	// Set timeout_ms to 2000ms (should be enough to catch the 200ms result)
	start := time.Now()
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":2000}`), "Queued")
	elapsed := time.Since(start)

	result := parseMCPResponseData(t, resp.Result)
	if result["status"] != "complete" {
		t.Errorf("Expected status=complete with timeout_ms=2000, got %v", result["status"])
	}
	if elapsed > 2*time.Second {
		t.Errorf("Should have completed well within 2s, took %v", elapsed)
	}
}

func TestMaybeWaitForCommand_TimeoutMs_ShortTimeout(t *testing.T) {
	// Not parallel: mutates package-level asyncPollInterval.
	prevPoll := asyncPollInterval
	asyncPollInterval = 50 * time.Millisecond
	defer func() {
		asyncPollInterval = prevPoll
	}()

	handler, _, cap := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	correlationID := "test-short-timeout-123"
	cap.RegisterCommand(correlationID, "q-short-123", 60*time.Second)

	// Connect extension (fast path — no long-poll)
	cap.SimulateExtensionConnectForTest()

	// Set a very short timeout_ms — command will not complete in time
	start := time.Now()
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":300}`), "Queued")
	elapsed := time.Since(start)

	result := parseMCPResponseData(t, resp.Result)

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
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":0}`), "Queued")

	result := parseMCPResponseData(t, resp.Result)
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

	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap}
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-async-275"

	// sync=false should return queued with correlation_id
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"sync":false}`), "Queued")

	result := parseMCPResponseData(t, resp.Result)
	if result["status"] != "queued" {
		t.Errorf("Expected status=queued with sync=false, got %v", result["status"])
	}
	if result["correlation_id"] != correlationID {
		t.Errorf("Expected correlation_id=%s, got %v", correlationID, result["correlation_id"])
	}
}

func TestMaybeWaitForCommand_TimeoutMs_NegativeIgnored(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap, coldStartTimeout: 0}
	req := mcp.JSONRPCRequest{ID: 1}
	correlationID := "test-neg-timeout-123"

	// Negative timeout_ms should be treated as default (not infinite)
	// Without extension, should fail fast
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"timeout_ms":-1}`), "Queued")

	result := parseMCPResponseData(t, resp.Result)
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
	cap.SimulateExtensionConnectForTest()

	// Complete the link_health command after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		// Find the pending command and complete it
		pending := cap.GetPendingCommands()
		for _, cmd := range pending {
			if cmd != nil && strings.HasPrefix(cmd.CorrelationID, "link_health_") {
				cap.CompleteCommand(cmd.CorrelationID, json.RawMessage(`{"success":true,"healthy":5,"broken":0}`), "")
				break
			}
		}
	}()

	// sync=true (default) should wait for the result
	args := json.RawMessage(`{"what":"link_health","domain":"example.com"}`)
	resp := handler.analyzeDispatcher.Handle(req, args)

	result := parseMCPResponseData(t, resp.Result)
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

	result := parseMCPResponseData(t, resp.Result)
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
	// Not parallel: mutates package-level asyncPollInterval.
	prevPoll := asyncPollInterval
	asyncPollInterval = 50 * time.Millisecond
	defer func() {
		asyncPollInterval = prevPoll
	}()

	handler, _, cap := makeToolHandler(t)
	handler.coldStartTimeout = 0
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "test-client"}

	// Connect extension (fast path — no long-poll)
	cap.SimulateExtensionConnectForTest()

	// Complete the command after 200ms
	go func() {
		time.Sleep(200 * time.Millisecond)
		pending := cap.GetPendingCommands()
		for _, cmd := range pending {
			if cmd != nil && strings.HasPrefix(cmd.CorrelationID, "dom_") {
				cap.CompleteCommand(cmd.CorrelationID, json.RawMessage(`{"success":true,"elements":[]}`), "")
				break
			}
		}
	}()

	// Set timeout_ms=1000 — should be enough to catch the 200ms result
	args := json.RawMessage(`{"what":"dom","selector":"div","timeout_ms":1000}`)
	resp := handler.analyzeDispatcher.Handle(req, args)

	result := parseMCPResponseData(t, resp.Result)
	status, _ := result["status"].(string)
	if status == "queued" {
		t.Error("With sync=true (default) and timeout_ms=1000, should not return queued")
	}
}

// findPendingCommandByPrefix finds a pending command's correlation ID by prefix
func findPendingCommandByPrefix(cap *capture.Capture, prefix string) *queries.CommandResult {
	pending := cap.GetPendingCommands()
	for _, cmd := range pending {
		if cmd != nil && strings.HasPrefix(cmd.CorrelationID, prefix) {
			return cmd
		}
	}
	return nil
}
