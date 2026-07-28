// Purpose: Unit tests for browser-agent tools core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_core_unit_test.go — Unit tests for ToolHandler getters.
package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

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
