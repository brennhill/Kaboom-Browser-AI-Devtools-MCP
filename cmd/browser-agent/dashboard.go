// Purpose: Serves embedded HTML dashboard, diagnostics, logs, setup, and docs pages at browser-accessible routes.
// Why: Provides a local web UI for inspecting server state without requiring MCP client tooling.

package main

import (
	_ "embed"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

//go:embed dashboard.html
var dashboardHTML []byte

//go:embed diagnostics.html
var diagnosticsHTML []byte

//go:embed logs.html
var logsHTML []byte

//go:embed setup.html
var setupHTML []byte

//go:embed docs.html
var docsHTML []byte

// handleDashboard serves the HTML dashboard for browser access.
// If the client sends Accept: application/json, falls back to the JSON discovery response.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Content negotiation: JSON for programmatic clients, HTML for browsers
	accept := r.Header.Get("Accept")
	if accept == "application/json" || (!strings.Contains(accept, "text/html") && strings.Contains(accept, "application/json")) {
		jsonResponse(w, http.StatusOK, map[string]string{
			"name":    mcpServerName,
			"version": version,
			"health":  "/health",
			"logs":    "/logs",
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(dashboardHTML); err != nil {
		stderrf("[Kaboom] failed to write dashboard response: %v\n", err)
	}
}

// handleStatusAPI serves GET /api/status with aggregated data for the dashboard.
func handleStatusAPI(server *Server, cap *capture.Store, mcpHandler *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		resp := map[string]any{
			"version":        version,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
			"pid":            os.Getpid(),
			"platform":       runtime.GOOS + "/" + runtime.GOARCH,
		}

		buffers := map[string]any{
			"console_entries":  server.logs.EntryCount(),
			"console_capacity": server.logs.MaxEntries(),
		}

		if cap != nil {
			snap := cap.GetHealthSnapshot()

			resp["extension_connected"] = cap.IsExtensionConnected()
			resp["pilot_enabled"] = snap.PilotEnabled
			if !snap.LastPollTime.IsZero() {
				resp["last_poll_at"] = snap.LastPollTime.Format(time.RFC3339)
			}

			buffers["network_entries"] = snap.NetworkBodyCount
			buffers["network_capacity"] = capture.MaxNetworkBodies
			buffers["websocket_entries"] = snap.WebSocketCount
			buffers["websocket_capacity"] = capture.MaxWSEvents
			buffers["action_entries"] = snap.ActionCount
			buffers["action_capacity"] = capture.MaxEnhancedActions

			resp["recent_commands"] = buildRecentCommands(cap.GetHTTPDebugLog())
		} else {
			resp["extension_connected"] = false
			resp["pilot_enabled"] = false
		}

		resp["buffers"] = buffers

		// Terminal server status
		termPort := server.getTerminalPort()
		termInfo := map[string]any{
			"port":     termPort,
			"running":  termPort > 0,
			"sessions": 0,
		}
		if server.ptyManager != nil {
			termInfo["sessions"] = server.ptyManager.Count()
			termInfo["session_ids"] = server.ptyManager.List()
		}
		resp["terminal"] = termInfo
		resp["listen_port"] = server.getListenPort()

		if mcpHandler != nil && mcpHandler.toolHandler != nil {
			if th, ok := mcpHandler.toolHandler.(*ToolHandler); ok && th.healthMetrics != nil {
				resp["audit"] = th.healthMetrics.BuildAuditInfo()
			}
		}

		jsonResponse(w, http.StatusOK, resp)
	}
}

// serveEmbeddedHTML is a helper that serves an embedded HTML page.
func serveEmbeddedHTML(w http.ResponseWriter, r *http.Request, content []byte, name string) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(content); err != nil {
		stderrf("[Kaboom] failed to write %s response: %v\n", name, err)
	}
}
