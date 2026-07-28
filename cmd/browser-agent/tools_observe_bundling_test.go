// Purpose: Tests for observe response bundling and batching.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_bundling_test.go — Tests for error bundling observe mode.
//
// Error bundles assemble complete debugging context in a single call:
// error + recent network requests + recent actions + recent logs.
//
// Run: go test ./cmd/browser-agent -run "TestErrorBundles" -v
package main

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// ============================================
// Test Infrastructure
// ============================================

type bundleTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

func newBundleTestEnv(t *testing.T) *bundleTestEnv {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test-error-bundles.jsonl", 1000)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)
	return &bundleTestEnv{handler: handler, server: server, capture: cap}
}

func (e *bundleTestEnv) addLogEntry(entry types.LogEntry) {
	e.server.logs.SeedEntries([]types.LogEntry{
		entry,
	}, []time.Time{
		time.Now(),
	})
}

func (e *bundleTestEnv) callErrorBundles(t *testing.T, args string) (mcp.MCPToolResult, bool) {
	t.Helper()
	rawArgs := json.RawMessage(args)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.GetErrorBundles(buildObserveReadDeps(e.handler), req, rawArgs)
	if resp.Result == nil {
		return mcp.MCPToolResult{}, false
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return result, true
}

// parseBundles extracts the bundles array from an MCP response
func (e *bundleTestEnv) parseBundles(t *testing.T, result mcp.MCPToolResult) []map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse JSON: %v\ntext: %s", err, text[:min(len(text), 200)])
	}
	bundlesRaw, ok := data["bundles"].([]any)
	if !ok {
		t.Fatal("expected 'bundles' array in response")
	}
	bundles := make([]map[string]any, len(bundlesRaw))
	for i, b := range bundlesRaw {
		bundles[i] = b.(map[string]any)
	}
	return bundles
}

// ============================================
// Empty State
// ============================================

func TestErrorBundles_EmptyBuffers(t *testing.T) {
	env := newBundleTestEnv(t)

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("error_bundles should return result even when empty")
	}
	if result.IsError {
		t.Error("empty buffers should NOT return isError")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 0 {
		t.Errorf("expected 0 bundles, got %d", len(bundles))
	}
}

// ============================================
// Error With No Context
// ============================================

func TestErrorBundles_ErrorWithNoContext(t *testing.T) {
	env := newBundleTestEnv(t)

	// Add an error with no matching network/actions/logs
	now := time.Now().UTC()
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "ReferenceError: foo is not defined",
		"source":    "app.js",
		"url":       "https://example.com/app.js",
		"line":      42,
		"column":    15,
		"stack":     "ReferenceError: foo is not defined\n    at bar (app.js:42)",
		"timestamp": now.Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}

	b := bundles[0]

	// Error should be populated
	errObj, ok := b["error"].(map[string]any)
	if !ok {
		t.Fatal("bundle should have 'error' object")
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "foo is not defined") {
		t.Errorf("error message mismatch: %s", msg)
	}

	// Context arrays should exist but be empty
	network, _ := b["network"].([]any)
	actions, _ := b["actions"].([]any)
	logs, _ := b["logs"].([]any)

	if len(network) != 0 {
		t.Errorf("expected 0 network entries, got %d", len(network))
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

// ============================================
// Error With Matching Network Body
// ============================================

func TestErrorBundles_ErrorWithNetwork(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add a network body 1 second before the error
	env.capture.Telemetry().AddNetworkBodiesForTest([]types.NetworkBody{
		{
			URL:          "https://api.example.com/users",
			Method:       "GET",
			Status:       500,
			ResponseBody: `{"error":"Internal Server Error"}`,
			Timestamp:    now.Add(-1 * time.Second).Format(time.RFC3339),
		},
	})

	// Add the error
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Failed to load users",
		"timestamp": now.Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}

	network, _ := bundles[0]["network"].([]any)
	if len(network) != 1 {
		t.Fatalf("expected 1 network entry in bundle, got %d", len(network))
	}

	entry := network[0].(map[string]any)
	if url, _ := entry["url"].(string); url != "https://api.example.com/users" {
		t.Errorf("network URL mismatch: %s", url)
	}
	if status, _ := entry["status"].(float64); status != 500 {
		t.Errorf("network status mismatch: %v", status)
	}
}

// ============================================
// Error With Matching Action
// ============================================

func TestErrorBundles_ErrorWithAction(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add an action 2 seconds before the error
	env.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{
			Type:      "click",
			Timestamp: now.Add(-2 * time.Second).UnixMilli(),
			URL:       "https://example.com",
			Selectors: map[string]any{"css": "button.submit"},
		},
	})

	// Add the error
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Submit failed",
		"timestamp": now.Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}

	actions, _ := bundles[0]["actions"].([]any)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action in bundle, got %d", len(actions))
	}

	action := actions[0].(map[string]any)
	if typ, _ := action["type"].(string); typ != "click" {
		t.Errorf("action type mismatch: %s", typ)
	}
}

// ============================================
// Error With Matching Log
// ============================================

func TestErrorBundles_ErrorWithLog(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add a warning log 1 second before the error
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "warn",
		"message":   "Cache miss for user profile",
		"timestamp": now.Add(-1 * time.Second).Format(time.RFC3339),
	})

	// Add the error
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "TypeError in UserProfile render",
		"timestamp": now.Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}

	logs, _ := bundles[0]["logs"].([]any)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log in bundle, got %d", len(logs))
	}

	logEntry := logs[0].(map[string]any)
	if msg, _ := logEntry["message"].(string); !strings.Contains(msg, "Cache miss") {
		t.Errorf("log message mismatch: %s", msg)
	}
}

// ============================================
// Window Boundary
// ============================================

func TestErrorBundles_WindowBoundary(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add action at T-2s (within 3s window) — should be included
	env.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{
			Type:      "click",
			Timestamp: now.Add(-2 * time.Second).UnixMilli(),
			Selectors: map[string]any{"css": "button.in-window"},
		},
	})

	// Add action at T-5s (outside 3s window) — should be excluded
	env.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{
			Type:      "click",
			Timestamp: now.Add(-5 * time.Second).UnixMilli(),
			Selectors: map[string]any{"css": "button.outside-window"},
		},
	})

	// Add the error
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Window test error",
		"timestamp": now.Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}

	actions, _ := bundles[0]["actions"].([]any)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action (in-window only), got %d", len(actions))
	}
}

// ============================================
// Custom Window
// ============================================

func TestErrorBundles_CustomWindow(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add action at T-4s
	env.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{
			Type:      "click",
			Timestamp: now.Add(-4 * time.Second).UnixMilli(),
			Selectors: map[string]any{"css": "button.test"},
		},
	})

	// Add the error
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Custom window test",
		"timestamp": now.Format(time.RFC3339),
	})

	// Default window (3s) should exclude the action at T-4s
	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}
	bundles := env.parseBundles(t, result)
	actions, _ := bundles[0]["actions"].([]any)
	if len(actions) != 0 {
		t.Errorf("default 3s window should exclude action at T-4s, got %d actions", len(actions))
	}

	// 5s window should include it
	result2, ok := env.callErrorBundles(t, `{"window_seconds": 5}`)
	if !ok {
		t.Fatal("should return result")
	}
	bundles2 := env.parseBundles(t, result2)
	actions2, _ := bundles2[0]["actions"].([]any)
	if len(actions2) != 1 {
		t.Errorf("5s window should include action at T-4s, got %d actions", len(actions2))
	}
}

// ============================================
// Limit
// ============================================

func TestErrorBundles_Limit(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add 5 errors
	for i := 0; i < 5; i++ {
		env.addLogEntry(types.LogEntry{
			"type":      "console",
			"level":     "error",
			"message":   "Error number",
			"timestamp": now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		})
	}

	// Limit to 2
	result, ok := env.callErrorBundles(t, `{"limit": 2}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 2 {
		t.Errorf("expected 2 bundles (limit=2), got %d", len(bundles))
	}
}

// ============================================
// Multiple Errors Share Context
// ============================================

func TestErrorBundles_SharedContext(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Add a network body
	env.capture.Telemetry().AddNetworkBodiesForTest([]types.NetworkBody{
		{
			URL:       "https://api.example.com/data",
			Method:    "GET",
			Status:    500,
			Timestamp: now.Add(-1 * time.Second).Format(time.RFC3339),
		},
	})

	// Add two errors within 1 second — both should see the same network body
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Error A",
		"timestamp": now.Format(time.RFC3339),
	})
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Error B",
		"timestamp": now.Add(500 * time.Millisecond).Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(bundles))
	}

	// Both bundles should contain the same network entry
	for i, b := range bundles {
		network, _ := b["network"].([]any)
		if len(network) != 1 {
			t.Errorf("bundle %d should have 1 network entry, got %d", i, len(network))
		}
	}
}

// ============================================
// Via Observe Dispatcher
// ============================================

func TestErrorBundles_ViaObserveDispatcher(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()
	env.addLogEntry(types.LogEntry{
		"type":      "console",
		"level":     "error",
		"message":   "Dispatcher test error",
		"timestamp": now.Format(time.RFC3339),
	})

	// Call via the observe dispatcher (as a real client would)
	args := json.RawMessage(`{"what":"error_bundles"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.observeDispatcher.Handle(req, args)

	if resp.Result == nil {
		t.Fatal("observe error_bundles should return result")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if result.IsError {
		t.Error("error_bundles should not return isError")
	}
	if !strings.Contains(result.Content[0].Text, "Dispatcher test error") {
		t.Error("response should contain the error message")
	}
}

// ============================================
// Extension entries use "ts" field
// ============================================

func TestErrorBundles_TsField(t *testing.T) {
	env := newBundleTestEnv(t)

	now := time.Now().UTC()

	// Extension-originated entries use "ts" instead of "timestamp"
	env.addLogEntry(types.LogEntry{
		"type":    "console",
		"level":   "error",
		"message": "Extension error with ts field",
		"ts":      now.Format(time.RFC3339),
	})

	result, ok := env.callErrorBundles(t, `{}`)
	if !ok {
		t.Fatal("should return result")
	}

	bundles := env.parseBundles(t, result)
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle from ts-field entry, got %d", len(bundles))
	}

	errObj, ok := bundles[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("bundle should have 'error' object")
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "ts field") {
		t.Errorf("error message mismatch: %s", msg)
	}
}

// ============================================
// No Panic Safety
// ============================================

func TestErrorBundles_NoPanic(t *testing.T) {
	env := newBundleTestEnv(t)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("error_bundles PANICKED: %v", r)
		}
	}()

	// Call with various args including edge cases
	for _, args := range []string{
		`{}`,
		`{"limit": 0}`,
		`{"limit": -1}`,
		`{"window_seconds": 0}`,
		`{"window_seconds": 100}`,
		`{"limit": 1, "window_seconds": 1}`,
	} {
		env.callErrorBundles(t, args)
	}
}
