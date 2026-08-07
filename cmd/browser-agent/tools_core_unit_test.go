// Purpose: Unit tests for browser-agent tools core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_core_unit_test.go — Unit tests for ToolHandler getters.
package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestToolHandlerRecordsUsageOutcomesAndSessionDepth(t *testing.T) {
	t.Parallel()

	handler := createTestToolHandler(t)
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
	mcpHandler := NewToolHandler(server, cap)
	if mcpHandler.tools.Capture != cap {
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
	mcpHandler := NewToolHandler(server, cap)
	limiter := mcpHandler.tools.Limiter
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
	mcpHandler := NewToolHandler(server, cap)
	if mcpHandler.tools.Redactor == nil {
		t.Fatal("MCP redaction engine should be configured")
	}
}
