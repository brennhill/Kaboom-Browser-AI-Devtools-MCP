// Purpose: Implements diagnostics response assembly for /diagnostics endpoints.
// Why: Keeps debug payload shaping separate from health/shutdown endpoint behavior.

package main

import (
	"net/http"
	"runtime"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// lastConsoleEvent returns a summary of the most recent console log entry.
func (s *Server) lastConsoleEvent() map[string]any {
	last, ok := s.logs.LastEntry()
	if !ok {
		return nil
	}
	args := last["args"]
	if argsSlice, ok := args.([]any); ok && len(argsSlice) > 0 {
		if str, ok := argsSlice[0].(string); ok && len(str) > 100 {
			args = str[:100] + "..."
		} else {
			args = argsSlice[0]
		}
	}
	return map[string]any{
		"level":   last["level"],
		"message": args,
		"ts":      last["ts"],
	}
}

// handleDiagnostics serves the /diagnostics endpoint with debug information.
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	now := time.Now()
	launch := getCurrentLaunchMode()
	resp := map[string]any{
		"generated_at":   now.Format(time.RFC3339),
		"version":        version,
		"uptime_seconds": int(now.Sub(startTime).Seconds()),
		"system": map[string]any{
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"go_version": runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
		},
		"logs": map[string]any{
			"entries":     s.logs.EntryCount(),
			"max_entries": s.logs.MaxEntries(),
			"log_file":    s.logs.LogFile(),
		},
		"launch_mode": map[string]any{
			"mode":             launch.Mode,
			"reason":           launch.Reason,
			"parent_process":   launch.ParentProcess,
			"is_tty":           launch.IsTTY,
			"strict_required":  launch.StrictRequired,
			"under_supervisor": launch.UnderSupervisor,
		},
	}

	if cap != nil {
		appendCaptureDiagnostics(resp, cap)
	}

	lastEvents := map[string]any{}
	if evt := s.lastConsoleEvent(); evt != nil {
		lastEvents["console"] = evt
	}
	resp["last_events"] = lastEvents

	if cap != nil {
		httpDebugLog := cap.GetHTTPDebugLog()
		resp["http_debug_log"] = map[string]any{
			"count":   len(httpDebugLog),
			"entries": httpDebugLog,
		}
	}

	jsonResponse(w, http.StatusOK, resp)
}

// appendCaptureDiagnostics adds capture-related diagnostic fields to response map.
func appendCaptureDiagnostics(resp map[string]any, cap *capture.Store) {
	snap := cap.GetHealthSnapshot()
	health := cap.GetHealthStatus()

	resp["buffers"] = map[string]any{
		"websocket_events": snap.WebSocketCount,
		"network_bodies":   snap.NetworkBodyCount,
		"actions":          snap.ActionCount,
		"pending_queries":  snap.PendingQueryCount,
		"query_results":    snap.QueryResultCount,
	}

	wsStatus := cap.GetWebSocketStatus(capture.WebSocketStatusFilter{})
	conns := make([]map[string]any, 0, len(wsStatus.Connections))
	for _, c := range wsStatus.Connections {
		conns = append(conns, map[string]any{
			"id":  c.ID,
			"url": c.URL,
		})
	}
	resp["websocket_connections"] = conns

	resp["config"] = map[string]any{
		"query_timeout": snap.QueryTimeout.String(),
	}

	const defaultTraceLimit = 25
	traces := cap.GetRecentCommandTraces(defaultTraceLimit)
	traceEntries := make([]map[string]any, 0, len(traces))
	for _, trace := range traces {
		if trace == nil {
			continue
		}
		traceID := trace.TraceID
		if traceID == "" {
			traceID = trace.CorrelationID
		}
		traceEntries = append(traceEntries, map[string]any{
			"trace_id":       traceID,
			"correlation_id": trace.CorrelationID,
			"query_id":       trace.QueryID,
			"status":         trace.Status,
			"timeline":       trace.TraceTimeline,
			"events":         trace.TraceEvents,
			"created_at":     trace.CreatedAt.Format(time.RFC3339),
			"updated_at":     trace.UpdatedAt.Format(time.RFC3339),
		})
	}
	resp["command_traces"] = map[string]any{
		"count":   len(traceEntries),
		"limit":   defaultTraceLimit,
		"entries": traceEntries,
	}

	lastPoll := any(nil)
	if !snap.LastPollTime.IsZero() {
		lastPoll = snap.LastPollTime.Format(time.RFC3339)
	}
	resp["extension"] = map[string]any{
		"polling":       !snap.LastPollTime.IsZero(),
		"last_poll_at":  lastPoll,
		"ext_session":   snap.ExtSessionID,
		"pilot_enabled": snap.PilotEnabled,
	}

	resp["circuit"] = map[string]any{
		"open":         snap.CircuitOpen,
		"current_rate": health.CurrentRate,
		"reason":       snap.CircuitReason,
	}
}
