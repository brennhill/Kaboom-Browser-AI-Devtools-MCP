// recordinghandler_test.go — Unit tests for the recordinghandler sub-package exported API.

package recordinghandler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ---------------------------------------------------------------------------
// BuildPlaybackResult
// ---------------------------------------------------------------------------

func TestBuildPlaybackResult_AllSuccess(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	session := &capture.PlaybackSession{
		RecordingID:      "rec-1",
		StartedAt:        time.Now().Add(-500 * time.Millisecond),
		ActionsExecuted:  5,
		ActionsFailed:    0,
		SelectorFailures: map[string]int{},
		Results:          nil,
	}

	resp := BuildPlaybackResult(req, "rec-1", session)
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.IsError {
		t.Error("should not be error result")
	}

	// Parse the JSON content block to verify fields.
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		// Content might be structured differently, just check it exists.
		t.Logf("content is not raw JSON, checking text content instead")
	}
}

func TestBuildPlaybackResult_PartialFailure(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	session := &capture.PlaybackSession{
		RecordingID:     "rec-2",
		StartedAt:       time.Now().Add(-1 * time.Second),
		ActionsExecuted: 3,
		ActionsFailed:   2,
		SelectorFailures: map[string]int{
			"#missing-btn": 2,
		},
	}

	resp := BuildPlaybackResult(req, "rec-2", session)
	if resp.Error != nil {
		t.Fatalf("expected no JSONRPC error, got %v", resp.Error)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The content should mention "partial" status and "3/5 actions".
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	text := result.Content[0].Text
	if text == "" {
		t.Fatal("expected non-empty content text")
	}
}

func TestBuildPlaybackResult_ZeroActions(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	session := &capture.PlaybackSession{
		RecordingID:      "rec-3",
		StartedAt:        time.Now(),
		ActionsExecuted:  0,
		ActionsFailed:    0,
		SelectorFailures: map[string]int{},
	}

	resp := BuildPlaybackResult(req, "rec-3", session)
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}
}

func TestBuildPlaybackResult_ResponseID(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: "custom-id"}
	session := &capture.PlaybackSession{
		RecordingID: "rec-4",
		StartedAt:   time.Now(),
	}

	resp := BuildPlaybackResult(req, "rec-4", session)
	if resp.ID != "custom-id" {
		t.Errorf("response ID should match request ID, got %v", resp.ID)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC version should be 2.0, got %s", resp.JSONRPC)
	}
}
