// owner_test.go — Tests per-client passive telemetry augmentation and eviction.

package mcptelemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestOwnerTracksDeltasPerClientAndHonorsModes(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	errorTotal := int64(3)
	mode := ModeAuto
	captured := capture.NewCapture()
	owner := New(Config{
		ErrorTotal: func() int64 { return errorTotal },
		Mode:       func() string { return mode },
		Now:        func() time.Time { return now },
	})
	owner.SetCapture(captured)

	first := owner.Augment(successResponse(t), "client-a", "observe", json.RawMessage(`{"what":"logs"}`))
	metadata := responseMetadata(t, first)
	if metadata["telemetry_changed"] != false {
		t.Fatalf("first telemetry_changed = %#v", metadata["telemetry_changed"])
	}
	if _, exists := metadata["telemetry_summary"]; exists {
		t.Fatal("auto mode included unchanged summary")
	}

	errorTotal++
	captured.Telemetry().AddNetworkBodies([]types.NetworkBody{{Method: "GET", URL: "https://example.test", Status: 500}})
	second := owner.Augment(successResponse(t), "client-a", "observe", nil)
	metadata = responseMetadata(t, second)
	if metadata["telemetry_changed"] != true {
		t.Fatalf("second telemetry_changed = %#v", metadata["telemetry_changed"])
	}
	summary := metadata["telemetry_summary"].(map[string]any)
	if summary["new_errors_since_last_call"] != float64(1) || summary["new_network_requests_since_last_call"] != float64(1) {
		t.Fatalf("summary = %#v", summary)
	}

	otherClient := owner.Augment(successResponse(t), "client-b", "observe", nil)
	if responseMetadata(t, otherClient)["telemetry_changed"] != false {
		t.Fatal("new client inherited another client's cursor")
	}

	mode = ModeFull
	full := owner.Augment(successResponse(t), "client-b", "observe", nil)
	if _, exists := responseMetadata(t, full)["telemetry_summary"]; !exists {
		t.Fatal("full mode omitted unchanged summary")
	}

	off := owner.Augment(successResponse(t), "client-b", "observe", json.RawMessage(`{"telemetry_mode":"off"}`))
	if _, exists := responseMetadata(t, off)["telemetry_changed"]; exists {
		t.Fatal("per-call off mode retained telemetry metadata")
	}
}

func TestOwnerEvictsOnlyExpiredCursorsWhenBoundIsExceeded(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	owner := New(Config{Now: func() time.Time { return now }})
	owner.cursors["expired"] = cursor{lastSeen: now.Add(-cursorTTL - time.Second)}
	owner.cursors["fresh"] = cursor{lastSeen: now}
	for index := 0; index < cursorMaxEntries; index++ {
		owner.cursors[string(rune(index+1000))] = cursor{lastSeen: now}
	}

	owner.evictExpiredLocked()

	if _, exists := owner.cursors["expired"]; exists {
		t.Fatal("expired cursor was retained")
	}
	if _, exists := owner.cursors["fresh"]; !exists {
		t.Fatal("fresh cursor was evicted")
	}
}

func successResponse(t *testing.T) mcp.JSONRPCResponse {
	t.Helper()
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: mcp.TextResponse("ok")}
}

func responseMetadata(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result.Metadata
}
