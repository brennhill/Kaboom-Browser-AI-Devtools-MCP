// Purpose: Implements server health, diagnostics, and shutdown HTTP endpoints.
// Why: Keeps operational endpoint behavior separate from route registration wiring.

package main

import (
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	// shutdownSignalDelay is the pause after sending the HTTP response before
	// sending SIGTERM, giving the response time to flush to the client.
	shutdownSignalDelay = 100 * time.Millisecond
)

// handleHealth serves the /health endpoint with server status and version info.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	logFileSize := int64(0)
	if fi, err := os.Stat(s.logs.logFile); err == nil {
		logFileSize = fi.Size()
	}

	availVer := getAvailableVersion()

	resp := map[string]any{
		"status":       "ok",
		"service-name": mcpServerName,
		"name":         mcpServerName,
		"version":      version,
		"logs": map[string]any{
			"entries":       s.logs.getEntryCount(),
			"max_entries":   s.logs.maxEntries,
			"log_file":      s.logs.logFile,
			"log_file_size": logFileSize,
			"dropped_count": s.logs.getLogDropCount(),
		},
	}
	s.addTerminalHealth(resp)

	successReads, failedReads := bridge.SnapshotFastPathResourceReadCounters()
	resp["bridge_fastpath"] = map[string]any{
		"resources_read_success": successReads,
		"resources_read_failure": failedReads,
	}
	if availVer != "" {
		resp["available_version"] = availVer
	}
	if info := buildUpgradeInfo(); info != nil {
		resp["upgrade_pending"] = info
	}
	if cap != nil {
		extStatus := cap.GetExtensionStatus()
		pilotStatus, _ := cap.GetPilotStatus().(map[string]any)
		pilotState, _ := pilotStatus["state"].(string)
		securityMode, productionParity, rewrites := cap.GetSecurityMode()
		resp["capture"] = map[string]any{
			"available":           true,
			"pilot_enabled":       cap.IsPilotActionAllowed(),
			"pilot_state":         pilotState,
			"extension_connected": cap.IsExtensionConnected(),
			"extension_last_seen": extStatus["last_seen"],
			"extension_client_id": extStatus["client_id"],
			"security_mode":       securityMode,
			"production_parity":   productionParity,
			"insecure_rewrites":   rewrites,
		}
	}
	jsonResponse(w, http.StatusOK, resp)
}

// handleShutdown initiates a graceful server shutdown via SIGTERM.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	_ = s.logs.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "shutdown_requested",
		"source":    "http",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "shutting_down",
		"message": "Server shutdown initiated",
	})

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	util.SafeGo(func() {
		time.Sleep(shutdownSignalDelay)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})
}

// addTerminalHealth writes the terminal server's state into a health payload.
//
// terminal_available is ALWAYS present so a caller can branch on it without having
// to infer meaning from an absent field — the old payload simply omitted
// terminal_port on a bind failure, which is indistinguishable from "this build has
// no terminal". The diagnosis fields appear only when something is wrong: noise on
// the happy path trains people to ignore the field.
func (s *Server) addTerminalHealth(resp map[string]any) {
	st := s.getTerminalStatus()
	resp["terminal_available"] = st.Available
	if st.Port > 0 {
		resp["terminal_port"] = st.Port
	}
	if st.Available {
		return
	}
	if st.Error != "" {
		resp["terminal_error"] = st.Error
	}
	if st.BlockedByPID > 0 || st.BlockedByCommand != "" {
		resp["terminal_blocked_by"] = map[string]any{
			"pid":     st.BlockedByPID,
			"command": st.BlockedByCommand,
		}
	}
}

// buildHealthPayload exposes the terminal portion of the health payload for tests.
func (s *Server) buildHealthPayload() map[string]any {
	resp := map[string]any{}
	s.addTerminalHealth(resp)
	return resp
}
