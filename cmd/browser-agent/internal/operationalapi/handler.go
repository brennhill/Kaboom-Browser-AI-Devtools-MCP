// handler.go — Serves operational health, diagnostics, shutdown, log, and debug routes.

package operationalapi

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const shutdownSignalDelay = 100 * time.Millisecond

// TerminalStatus is the operational terminal state exposed by health routes.
type TerminalStatus struct {
	Available      bool
	Port           int
	Error          string
	BlockedByPID   int
	BlockedCommand string
}

// Options supplies the runtime-owned state needed by operational endpoints.
type Options struct {
	Logs             *logstore.Store
	Capture          *capture.Capture
	Version          string
	StartedAt        time.Time
	TerminalStatus   func() TerminalStatus
	AvailableVersion func() string
	UpgradeInfo      func() *health.UpgradeInfo
	UsageTracker     func() *telemetry.UsageTracker
	MaxPostBodySize  int64
}

// Handler owns non-MCP operational HTTP endpoints.
type Handler struct {
	options Options
}

func New(options Options) *Handler {
	return &Handler{options: options}
}

func (h *Handler) ServeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	logFileSize := int64(0)
	if info, err := os.Stat(h.options.Logs.LogFile()); err == nil {
		logFileSize = info.Size()
	}
	response := map[string]any{
		"status": "ok", "name": identity.MCPServerName, "version": h.options.Version,
		"logs": map[string]any{
			"entries": h.options.Logs.EntryCount(), "max_entries": h.options.Logs.MaxEntries(),
			"log_file": h.options.Logs.LogFile(), "log_file_size": logFileSize, "dropped_count": h.options.Logs.DropCount(),
		},
	}
	h.addTerminalHealth(response)
	successReads, failedReads := bridge.SnapshotFastPathResourceReadCounters()
	response["bridge_fastpath"] = map[string]any{"resources_read_success": successReads, "resources_read_failure": failedReads}
	if availableVersion := h.availableVersion(); availableVersion != "" {
		response["available_version"] = availableVersion
	}
	if info := h.upgradeInfo(); info != nil {
		response["upgrade_pending"] = info
	}
	if h.options.Capture != nil {
		extension := h.options.Capture.Extension().GetExtensionStatus()
		pilot, _ := h.options.Capture.Extension().GetPilotStatus().(map[string]any)
		pilotState, _ := pilot["state"].(string)
		securityMode, productionParity, rewrites := h.options.Capture.Extension().GetSecurityMode()
		response["capture"] = map[string]any{
			"available": true, "pilot_enabled": h.options.Capture.Extension().IsPilotActionAllowed(), "pilot_state": pilotState,
			"extension_connected": h.options.Capture.Extension().IsExtensionConnected(), "extension_last_seen": extension["last_seen"],
			"extension_client_id": extension["client_id"], "security_mode": securityMode,
			"production_parity": productionParity, "insecure_rewrites": rewrites,
		}
	}
	httpapi.JSON(w, http.StatusOK, response)
}

func (h *Handler) availableVersion() string {
	if h.options.AvailableVersion == nil {
		return ""
	}
	return h.options.AvailableVersion()
}

func (h *Handler) upgradeInfo() *health.UpgradeInfo {
	if h.options.UpgradeInfo == nil {
		return nil
	}
	return h.options.UpgradeInfo()
}

func (h *Handler) ServeShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	_ = h.options.Logs.AppendToFile([]types.LogEntry{{
		"type": "lifecycle", "event": "shutdown_requested", "source": "http",
		"pid": os.Getpid(), "timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	httpapi.JSON(w, http.StatusOK, map[string]string{"status": "shutting_down", "message": "Server shutdown initiated"})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	util.SafeGo(func() {
		time.Sleep(shutdownSignalDelay)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGTERM)
	})
}

func (h *Handler) addTerminalHealth(response map[string]any) {
	status := TerminalStatus{}
	if h.options.TerminalStatus != nil {
		status = h.options.TerminalStatus()
	}
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
	if status.BlockedByPID > 0 || status.BlockedCommand != "" {
		response["terminal_blocked_by"] = map[string]any{"pid": status.BlockedByPID, "command": status.BlockedCommand}
	}
}

func (h *Handler) HealthPayload() map[string]any {
	response := map[string]any{}
	h.addTerminalHealth(response)
	return response
}

// lastConsoleEvent returns a summary of the most recent console log entry.
func (h *Handler) lastConsoleEvent() map[string]any {
	last, ok := h.options.Logs.LastEntry()
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
func (h *Handler) ServeDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	now := time.Now()
	launch := launchmode.Current()
	resp := map[string]any{
		"generated_at":   now.Format(time.RFC3339),
		"version":        h.options.Version,
		"uptime_seconds": int(now.Sub(h.options.StartedAt).Seconds()),
		"system": map[string]any{
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"go_version": runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
		},
		"logs": map[string]any{
			"entries":     h.options.Logs.EntryCount(),
			"max_entries": h.options.Logs.MaxEntries(),
			"log_file":    h.options.Logs.LogFile(),
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

	if h.options.Capture != nil {
		appendCaptureDiagnostics(resp, h.options.Capture)
	}

	lastEvents := map[string]any{}
	if evt := h.lastConsoleEvent(); evt != nil {
		lastEvents["console"] = evt
	}
	resp["last_events"] = lastEvents

	if h.options.Capture != nil {
		httpDebugLog := h.options.Capture.DiagnosticLogs().HTTPEntries()
		resp["http_debug_log"] = map[string]any{
			"count":   len(httpDebugLog),
			"entries": httpDebugLog,
		}
	}

	httpapi.JSON(w, http.StatusOK, resp)
}

// appendCaptureDiagnostics adds capture-related diagnostic fields to response map.
func appendCaptureDiagnostics(resp map[string]any, cap *capture.Capture) {
	snap := cap.GetHealthSnapshot()
	health := cap.Circuit().GetHealthStatus()

	resp["buffers"] = map[string]any{
		"websocket_events": snap.WebSocketCount,
		"network_bodies":   snap.NetworkBodyCount,
		"actions":          snap.ActionCount,
		"pending_queries":  snap.PendingQueryCount,
		"query_results":    snap.QueryResultCount,
	}

	wsStatus := cap.Telemetry().GetWebSocketStatus(types.WebSocketStatusFilter{})
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
	traces := cap.Queries().GetRecentCommandTraces(defaultTraceLimit)
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
func (h *Handler) ServeLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleLogsPost(w, r)
	case http.MethodDelete:
		h.options.Logs.ClearEntries()
		httpapi.JSON(w, http.StatusOK, map[string]bool{"cleared": true})
	default:
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (h *Handler) handleLogsPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.options.MaxPostBodySize)
	var body struct {
		Entries []types.LogEntry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Entries == nil {
		httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
		return
	}

	valid, rejected := logstore.ValidateEntries(body.Entries)
	received := h.options.Logs.AddEntries(valid)
	httpapi.JSON(w, http.StatusOK, map[string]int{
		"received": received,
		"rejected": rejected,
		"entries":  h.options.Logs.EntryCount(),
	})
}

// debugEndpointsEnabled reports whether non-production telemetry inspection routes are enabled.
func DebugEndpointsEnabled() bool {
	return os.Getenv("KABOOM_DEBUG") == "1"
}

func (h *Handler) ServeDebugUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	tracker := h.usageTracker()
	if tracker == nil {
		httpapi.JSON(w, http.StatusOK, map[string]any{"counts": map[string]int{}})
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]any{"counts": tracker.Peek()})
}

// handleDebugBeaconFlush returns the usage payload that a beacon would send without transmitting it.
func (h *Handler) ServeDebugBeaconFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	tracker := h.usageTracker()
	if tracker == nil {
		httpapi.JSON(w, http.StatusOK, map[string]any{
			"payload": nil,
			"flushed": 0,
			"message": "no usage tracker available",
		})
		return
	}
	snapshot := tracker.SwapAndReset()
	if snapshot == nil {
		httpapi.JSON(w, http.StatusOK, map[string]any{
			"payload": nil,
			"flushed": 0,
			"message": "no activity since last flush",
		})
		return
	}
	payload := telemetry.BuildUsageSummaryPayload(0, snapshot)
	httpapi.JSON(w, http.StatusOK, map[string]any{
		"payload": payload,
		"flushed": len(snapshot.ToolStats),
	})
}

func (h *Handler) usageTracker() *telemetry.UsageTracker {
	if h.options.UsageTracker == nil {
		return nil
	}
	return h.options.UsageTracker()
}
