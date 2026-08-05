// handlers.go — Implements passive network traffic recording handler.
// Why: Captures network baseline snapshots so agents can compare traffic patterns before and after code changes.
// Docs: docs/features/feature/backend-log-streaming/index.md

package netrecord

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/bodystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleNetworkRecording handles configure(what="network_recording").
func HandleNetworkRecording(bodies *bodystore.Store, state *NetworkRecordingState, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Operation string `json:"operation"`
		Domain    string `json:"domain"`
		Method    string `json:"method"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
		}
	}

	switch params.Operation {
	case "start":
		startedAt, ok := state.TryStart(params.Domain, params.Method)
		if !ok {
			return mcp.Fail(req, mcp.ErrInvalidParam,
				"Network recording already active",
				"Stop the current recording first with operation='stop'.")
		}
		result := map[string]any{
			"status":     "recording",
			"started_at": startedAt.Format(time.RFC3339),
		}
		if params.Domain != "" {
			result["domain_filter"] = params.Domain
		}
		if params.Method != "" {
			result["method_filter"] = params.Method
		}
		return mcp.Succeed(req, "Network recording started", result)

	case "stop":
		snap, wasActive := state.Stop()
		if !wasActive {
			return mcp.Fail(req, mcp.ErrInvalidParam,
				"No active network recording",
				"Start a recording first with operation='start'.")
		}

		// Collect network bodies captured since start time.
		recorded := CollectRecordedRequests(bodies.Snapshot().Bodies, snap)

		duration := time.Since(snap.StartTime)
		result := map[string]any{
			"status":      "stopped",
			"duration_ms": int(duration.Milliseconds()),
			"requests":    recorded,
			"count":       len(recorded),
		}
		return mcp.Succeed(req, "Network recording stopped", result)

	case "status", "":
		snap := state.Info()
		result := map[string]any{
			"active": snap.Active,
		}
		if snap.Active {
			result["started_at"] = snap.StartTime.Format(time.RFC3339)
			result["duration_ms"] = int(time.Since(snap.StartTime).Milliseconds())
			if snap.Domain != "" {
				result["domain_filter"] = snap.Domain
			}
			if snap.Method != "" {
				result["method_filter"] = snap.Method
			}
		}
		return mcp.Succeed(req, "Network recording status", result)

	default:
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"Unknown operation: "+params.Operation,
			"Use 'start', 'stop', or 'status'.")
	}
}
