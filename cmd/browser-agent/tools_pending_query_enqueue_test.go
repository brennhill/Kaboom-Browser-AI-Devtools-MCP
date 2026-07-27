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

func saturatePendingQueryQueue(t *testing.T, cap *capture.Store) {
	t.Helper()
	for i := 0; i < queries.MaxPendingQueries; i++ {
		_, err := cap.CreatePendingQueryWithTimeout(
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
	resp := inspect.HandleDOM(env.handler, req, json.RawMessage(`{"selector":"#target"}`))
	result := parseToolResult(t, resp)
	assertStructuredErrorCode(t, "toolQueryDOM queue full", result, mcp.ErrQueueFull)
}

func TestInteractNavigate_QueueFullFailsFast(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	mockConnectedTrackedTab(t, env.capture)
	saturatePendingQueryQueue(t, env.capture)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.interactAction().HandleBrowserActionNavigateImpl(req, json.RawMessage(`{"url":"https://example.com"}`))
	result := parseToolResult(t, resp)
	assertStructuredErrorCode(t, "interact navigate queue full", result, mcp.ErrQueueFull)
}
