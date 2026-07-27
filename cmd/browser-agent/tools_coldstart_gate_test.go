// tools_coldstart_gate_test.go — Tests for cold-start readiness gate in requireExtension.
// Why: Validates that interact commands wait for extension connection during cold starts.
// Docs: docs/features/feature/cold-start-queuing/index.md

package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Cold-start gate: requireExtension waits
// ============================================

func TestRequireExtension_ColdStart_WaitsForConnection(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.handler.Guards.SetExtensionReadinessTimeout(500 * time.Millisecond)

	// Simulate extension connecting after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		env.capture.SimulateExtensionConnectForTest()
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	start := time.Now()
	_, blocked := env.handler.Guards.RequireExtension(req)
	elapsed := time.Since(start)

	if blocked {
		t.Fatal("expected requireExtension to pass after waiting for connection")
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("expected some wait time, got %v", elapsed)
	}
	if elapsed > 450*time.Millisecond {
		t.Fatalf("waited too long: %v (connection fires at 100ms)", elapsed)
	}
}

func TestRequireExtension_ColdStart_TimesOut(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.handler.Guards.SetExtensionReadinessTimeout(200 * time.Millisecond)

	// Extension never connects
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	start := time.Now()
	resp, blocked := env.handler.Guards.RequireExtension(req)
	elapsed := time.Since(start)

	if !blocked {
		t.Fatal("expected requireExtension to block after cold-start timeout expires")
	}
	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected error code %q, got %q", mcp.ErrNoData, code)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("should have waited near timeout, only waited %v", elapsed)
	}
}

func TestRequireExtension_AlreadyConnected_NoWait(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)
	env.handler.Guards.SetExtensionReadinessTimeout(5 * time.Second)

	// Even with a long timeout, already-connected should be instant
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	start := time.Now()
	_, blocked := env.handler.Guards.RequireExtension(req)
	elapsed := time.Since(start)

	if blocked {
		t.Fatal("expected requireExtension to pass instantly when already connected")
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("already connected should be instant, took %v", elapsed)
	}
}

// ============================================
// MaybeWaitForCommand — instant extension check (P1-2: no double wait)
// ============================================

// After P1-2, MaybeWaitForCommand does an instant IsExtensionConnected() check
// instead of a blocking WaitForExtensionConnected. The cold-start gate is only
// in requireExtension, which runs before MaybeWaitForCommand in every handler.

func TestMaybeWaitForCommand_ExtensionConnected_WaitsForResult(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap, coldStartTimeout: 500 * time.Millisecond}
	correlationID := "test-connected-result"
	cap.RegisterCommand(correlationID, "q-connected-result", 15*time.Second)

	// Extension is already connected (requireExtension would have passed)
	cap.SimulateExtensionConnectForTest()

	// Complete the command after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cap.CompleteCommand(correlationID, json.RawMessage(`{"success":true}`), "")
	}()

	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	start := time.Now()
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{}`), "Queued")
	elapsed := time.Since(start)

	result := parseMCPResponseData(t, resp.Result)
	if result["status"] != "complete" {
		t.Errorf("expected status complete, got %v", result["status"])
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("expected some wait for command completion, got %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("took too long: %v (result at 100ms)", elapsed)
	}
}

func TestMaybeWaitForCommand_ExtensionNotConnected_InstantError(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap, coldStartTimeout: 200 * time.Millisecond}
	correlationID := "test-instant-fail"
	cap.RegisterCommand(correlationID, "q-instant-fail", 15*time.Second)

	// Extension NOT connected — MaybeWaitForCommand does instant check (no blocking wait)
	req := mcp.JSONRPCRequest{ID: 1, ClientID: "test-client"}
	start := time.Now()
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{}`), "Queued")
	elapsed := time.Since(start)

	// Should get instant no_data error (no blocking wait)
	se := extractStructuredErrorJSON(t, resp.Result)
	if se["error_code"] != mcp.ErrNoData {
		t.Errorf("expected error code %q, got %v", mcp.ErrNoData, se["error_code"])
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("should be instant (no blocking wait), took %v", elapsed)
	}
}

func TestMaybeWaitForCommand_Background_SkipsColdStartGate(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	handler := &ToolHandler{capture: cap, coldStartTimeout: 5 * time.Second}
	correlationID := "test-bg-skip"

	// Extension NOT connected, background mode
	req := mcp.JSONRPCRequest{ID: 1}
	start := time.Now()
	resp := handler.MaybeWaitForCommand(req, correlationID, json.RawMessage(`{"background":true}`), "Queued")
	elapsed := time.Since(start)

	result := parseMCPResponseData(t, resp.Result)
	if result["status"] != "queued" {
		t.Errorf("expected status queued for background, got %v", result["status"])
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("background mode should skip cold-start gate, took %v", elapsed)
	}
}
