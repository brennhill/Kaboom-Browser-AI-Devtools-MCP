// observehandler_test.go — Unit tests for the observehandler sub-package exported API.

package observehandler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ---------------------------------------------------------------------------
// ServerSideObserveModes
// ---------------------------------------------------------------------------

func TestServerSideObserveModes_NonEmpty(t *testing.T) {
	if len(ServerSideObserveModes) == 0 {
		t.Fatal("ServerSideObserveModes should not be empty")
	}
}

func TestServerSideObserveModes_KnownModes(t *testing.T) {
	expected := []string{
		"command_result", "pending_commands", "failed_commands",
		"saved_videos", "recordings", "inbox", "pilot",
		"annotations", "annotation_detail",
	}
	for _, mode := range expected {
		if !ServerSideObserveModes[mode] {
			t.Errorf("expected mode %q to be in ServerSideObserveModes", mode)
		}
	}
}

func TestServerSideObserveModes_LiveModesNotIncluded(t *testing.T) {
	// Extension-dependent modes should NOT be in the server-side map.
	liveModes := []string{"errors", "console", "network", "actions", "screenshot"}
	for _, mode := range liveModes {
		if ServerSideObserveModes[mode] {
			t.Errorf("live mode %q should NOT be in ServerSideObserveModes", mode)
		}
	}
}

// ---------------------------------------------------------------------------
// PrependDisconnectWarning
// ---------------------------------------------------------------------------

func TestPrependDisconnectWarning(t *testing.T) {
	// Build a basic MCP response with content.
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	original := mcp.Succeed(req, "test", map[string]any{"data": "value"})

	modified := PrependDisconnectWarning(original)

	// Parse the result to check the warning was prepended.
	var result mcp.MCPToolResult
	if err := json.Unmarshal(modified.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	if !strings.Contains(result.Content[0].Text, "Extension is not connected") {
		t.Error("first content block should contain disconnect warning")
	}
}

// ---------------------------------------------------------------------------
// AppendAlertsToResponse
// ---------------------------------------------------------------------------

func TestAppendAlertsToResponse(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	original := mcp.Succeed(req, "test", map[string]any{"data": "value"})

	alerts := []types.Alert{
		{Severity: "warning", Category: "regression", Title: "Error spike detected", Timestamp: "2026-03-29T12:00:00Z"},
	}

	modified := AppendAlertsToResponse(original, alerts)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(modified.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Should have at least 2 content blocks (original + alerts).
	if len(result.Content) < 2 {
		t.Fatalf("expected at least 2 content blocks, got %d", len(result.Content))
	}

	// Last block should contain alert text.
	lastBlock := result.Content[len(result.Content)-1]
	if lastBlock.Type != "text" {
		t.Errorf("last block type should be text, got %s", lastBlock.Type)
	}
	if !strings.Contains(lastBlock.Text, "Error spike detected") {
		t.Error("last block should contain alert title")
	}
}

func TestAppendAlertsToResponse_EmptyAlerts(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	original := mcp.Succeed(req, "test", map[string]any{"data": "value"})

	modified := AppendAlertsToResponse(original, nil)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(modified.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Should still have content blocks (original + empty alerts block).
	if len(result.Content) < 1 {
		t.Fatal("should have at least the original content block")
	}
}
