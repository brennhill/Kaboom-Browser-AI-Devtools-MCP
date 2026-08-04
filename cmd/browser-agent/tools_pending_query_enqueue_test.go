// Purpose: Regression tests for fail-fast queue saturation handling in tool enqueue paths.

package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/inspect"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func saturatePendingQueryQueue(t *testing.T, cap *capture.Capture) {
	t.Helper()
	for i := 0; i < queries.MaxPendingQueries; i++ {
		_, err := cap.Queries().CreatePendingQueryWithTimeout(
			queries.PendingQuery{
				Type:          "queue_saturation_test",
				Params:        json.RawMessage(`{"ok":true}`),
				CorrelationID: fmt.Sprintf("queue-saturation-%d", i),
			},
			30*time.Second,
			"",
		)
		if err != nil {
			t.Fatalf("failed to prefill queue at %d: %v", i, err)
		}
	}
}

func TestToolQueryDOM_QueueFullFailsFast(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)
	saturatePendingQueryQueue(t, env.capture)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := inspect.HandleDOM(buildInspectDeps(env.handler), req, json.RawMessage(`{"selector":"#target"}`))
	result := parseToolResult(t, resp)
	assertStructuredErrorCode(t, "toolQueryDOM queue full", result, mcp.ErrQueueFull)
}

func TestInteractNavigate_QueueFullFailsFast(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, env.capture)
	saturatePendingQueryQueue(t, env.capture)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.browserActions.Handle("navigate", req, json.RawMessage(`{"url":"https://example.com"}`))
	result := parseToolResult(t, resp)
	assertStructuredErrorCode(t, "interact navigate queue full", result, mcp.ErrQueueFull)
}

func TestInteractNavigate_QueueRecoversWithoutDiscardingAcceptedCommands(t *testing.T) {
	env := newToolTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, env.capture)
	saturatePendingQueryQueue(t, env.capture)

	env.capture.Queries().ExpireAllPendingQueries("pressure_fixture_released")
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.browserActions.Handle("navigate", req, json.RawMessage(`{"url":"https://example.com","sync":false}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("first healthy command after pressure recovery failed: %+v", result)
	}
	if got := env.capture.Queries().QueueDepth(); got != 1 {
		t.Fatalf("queue depth after recovery = %d, want exactly one accepted command", got)
	}
}
