// server_routes_diagnostics.go — Serves operational health, diagnostics, and shutdown routes.

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const shutdownSignalDelay = 100 * time.Millisecond

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	logFileSize := int64(0)
	if info, err := os.Stat(s.logs.LogFile()); err == nil {
		logFileSize = info.Size()
	}
	response := map[string]any{
		"status": "ok", "service-name": mcpServerName, "name": mcpServerName, "version": version,
		"logs": map[string]any{
			"entries": s.logs.EntryCount(), "max_entries": s.logs.MaxEntries(),
			"log_file": s.logs.LogFile(), "log_file_size": logFileSize, "dropped_count": s.logs.DropCount(),
		},
	}
	s.addTerminalHealth(response)
	successReads, failedReads := bridge.SnapshotFastPathResourceReadCounters()
	response["bridge_fastpath"] = map[string]any{"resources_read_success": successReads, "resources_read_failure": failedReads}
	if availableVersion := getAvailableVersion(); availableVersion != "" {
		response["available_version"] = availableVersion
	}
	if info := buildUpgradeInfo(); info != nil {
		response["upgrade_pending"] = info
	}
	if cap != nil {
		extension := cap.GetExtensionStatus()
		pilot, _ := cap.GetPilotStatus().(map[string]any)
		pilotState, _ := pilot["state"].(string)
		securityMode, productionParity, rewrites := cap.GetSecurityMode()
		response["capture"] = map[string]any{
			"available": true, "pilot_enabled": cap.IsPilotActionAllowed(), "pilot_state": pilotState,
			"extension_connected": cap.IsExtensionConnected(), "extension_last_seen": extension["last_seen"],
			"extension_client_id": extension["client_id"], "security_mode": securityMode,
			"production_parity": productionParity, "insecure_rewrites": rewrites,
		}
	}
	jsonResponse(w, http.StatusOK, response)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	_ = s.logs.AppendToFile([]LogEntry{{
		"type": "lifecycle", "event": "shutdown_requested", "source": "http",
		"pid": os.Getpid(), "timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	jsonResponse(w, http.StatusOK, map[string]string{"status": "shutting_down", "message": "Server shutdown initiated"})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	util.SafeGo(func() {
		time.Sleep(shutdownSignalDelay)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGTERM)
	})
}

func (s *Server) addTerminalHealth(response map[string]any) {
	status := s.getTerminalStatus()
	response["terminal_available"] = status.Available
	if status.Port > 0 {
		response["terminal_port"] = status.Port
	}
	if status.Available {
		return
	}
	if status.Error != "" {
		response["terminal_error"] = status.Error
	}
	if status.BlockedByPID > 0 || status.BlockedByCommand != "" {
		response["terminal_blocked_by"] = map[string]any{"pid": status.BlockedByPID, "command": status.BlockedByCommand}
	}
}

func (s *Server) buildHealthPayload() map[string]any {
	response := map[string]any{}
	s.addTerminalHealth(response)
	return response
}

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

// handleLogs ingests or clears operational log entries. Reads use /telemetry?type=logs.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleLogsPost(w, r)
	case http.MethodDelete:
		s.logs.ClearEntries()
		jsonResponse(w, http.StatusOK, map[string]bool{"cleared": true})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (s *Server) handleLogsPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		Entries []LogEntry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Entries == nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
		return
	}

	valid, rejected := logstore.ValidateEntries(body.Entries)
	received := s.logs.AddEntries(valid)
	jsonResponse(w, http.StatusOK, map[string]int{
		"received": received,
		"rejected": rejected,
		"entries":  s.logs.EntryCount(),
	})
}

// debugEndpointsEnabled reports whether non-production telemetry inspection routes are enabled.
func debugEndpointsEnabled() bool {
	return os.Getenv("KABOOM_DEBUG") == "1"
}

func handleDebugUsage(mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		tracker := mcp.GetUsageTracker()
		if tracker == nil {
			jsonResponse(w, http.StatusOK, map[string]any{"counts": map[string]int{}})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"counts": tracker.Peek()})
	}
}

// handleDebugBeaconFlush returns the usage payload that a beacon would send without transmitting it.
func handleDebugBeaconFlush(mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		tracker := mcp.GetUsageTracker()
		if tracker == nil {
			jsonResponse(w, http.StatusOK, map[string]any{
				"payload": nil,
				"flushed": 0,
				"message": "no usage tracker available",
			})
			return
		}
		snapshot := tracker.SwapAndReset()
		if snapshot == nil {
			jsonResponse(w, http.StatusOK, map[string]any{
				"payload": nil,
				"flushed": 0,
				"message": "no activity since last flush",
			})
			return
		}
		payload := telemetry.BuildUsageSummaryPayload(0, snapshot)
		jsonResponse(w, http.StatusOK, map[string]any{
			"payload": payload,
			"flushed": len(snapshot.ToolStats),
		})
	}
}
