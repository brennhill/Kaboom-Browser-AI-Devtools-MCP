// Purpose: Unit tests for browser-agent tools core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_core_unit_test.go — Unit tests for ToolHandler getters.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestNewToolHandlerUsesServerSessionProjectPath(t *testing.T) {
	t.Parallel()
	projectPath := t.TempDir()
	server, err := NewServer(filepath.Join(t.TempDir(), "test.jsonl"), 10)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(server.Close)
	server.sessionProjectPath = projectPath
	handler := NewToolHandler(server, capture.NewCapture())
	if handler.sessionStoreImpl == nil {
		t.Fatal("session store was not initialized")
	}
	if err := handler.sessionStoreImpl.Save("saved_states", "isolated", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	projectDir, err := state.ProjectDir(projectPath)
	if err != nil {
		t.Fatalf("ProjectDir() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "saved_states", "isolated.json")); err != nil {
		t.Fatalf("isolated state missing: %v", err)
	}
}

func TestToolHandlerRecordsUsageOutcomesAndSessionDepth(t *testing.T) {
	t.Parallel()

	handler, _, _ := makeToolHandler(t)
	tracker := telemetry.NewUsageTracker()
	handler.usageTracker = tracker
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call"}

	if _, handled := handler.HandleToolCall(request, "observe", json.RawMessage(`{"what":"errors"}`)); !handled {
		t.Fatal("observe was not handled")
	}
	if _, handled := handler.HandleToolCall(request, "interact", json.RawMessage(`{}`)); !handled {
		t.Fatal("interact was not handled")
	}
	counts := tracker.DebugCounts()
	if counts["observe:errors"] != 1 || counts["interact:unknown"] != 1 || counts["err:interact:unknown"] != 1 {
		t.Fatalf("usage counts = %#v", counts)
	}
	if tracker.SessionDepth() != 2 {
		t.Fatalf("session depth = %d, want 2", tracker.SessionDepth())
	}
	snapshot := tracker.SwapAndReset()
	if snapshot == nil || len(snapshot.ToolStats) == 0 {
		t.Fatalf("usage snapshot = %#v", snapshot)
	}
}

func TestMCPCaptureConfigured(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer(t.TempDir()+"/test-getters.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := NewToolHandler(server, cap)
	if handler.capture != cap {
		t.Fatal("MCP handler should retain the injected capture")
	}
}

func TestNewToolHandlerWiresCanonicalFiveToolCatalog(t *testing.T) {
	t.Parallel()
	environment := newToolTestEnv(t)
	for _, name := range []string{"observe", "analyze", "generate", "configure", "interact"} {
		module, ok := environment.handler.toolCatalog.Get(name)
		if !ok || module == nil || module.Describe().Name != name || len(module.Examples()) == 0 {
			t.Errorf("tool catalog module %q = %#v, %t", name, module, ok)
		}
	}
}

func TestMCPToolCallLimiterConfigured(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer(t.TempDir()+"/test-limiter.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := NewToolHandler(server, cap)
	limiter := handler.toolCallLimiter
	if limiter == nil {
		t.Fatal("MCP tool call limiter should be configured")
	}
	// Limiter should allow calls
	if !limiter.Allow() {
		t.Fatal("fresh limiter should allow first call")
	}
}

func TestMCPRedactionEngineConfigured(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer(t.TempDir()+"/test-redaction.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := NewToolHandler(server, cap)
	if handler.redactionEngine == nil {
		t.Fatal("MCP redaction engine should be configured")
	}
}

func TestHealthResponseIncludesCommandExecution(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()
	captured := capture.NewCapture()
	captured.Queries().RegisterCommand("warn-timeout", "query-warn-timeout", time.Minute)
	captured.Queries().ApplyCommandResult("warn-timeout", "timeout", nil, "synthetic-timeout")

	response := getHealthResponse(hm, captured, nil, nil, nil, "test")
	if response.CommandExecution.Status != "warn" || response.CommandExecution.Ready {
		t.Fatalf("command execution = %#v, want non-ready warning", response.CommandExecution)
	}
	if response.CommandExecution.RecentTimeoutCount != 1 {
		t.Fatalf("recent timeout count = %d, want 1", response.CommandExecution.RecentTimeoutCount)
	}
}
