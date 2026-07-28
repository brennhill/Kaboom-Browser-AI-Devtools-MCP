// Purpose: Serves embedded HTML dashboard, diagnostics, logs, setup, and docs pages at browser-accessible routes.
// Why: Provides a local web UI for inspecting server state without requiring MCP client tooling.

package dashboard

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
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

type JSONResponse func(http.ResponseWriter, int, any)

type RootOptions struct {
	Name         string
	Version      string
	JSONResponse JSONResponse
}

type StatusOptions struct {
	Version      string
	StartedAt    time.Time
	Capture      *capture.Capture
	Logs         func() (entries, capacity int)
	Terminal     func() (port, sessions int, sessionIDs []string)
	ListenPort   func() int
	Audit        func() any
	JSONResponse JSONResponse
}

// Root serves the HTML dashboard with a JSON discovery fallback.
func Root(options RootOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			options.JSONResponse(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		if r.Method != http.MethodGet {
			options.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			options.JSONResponse(w, http.StatusOK, map[string]string{
				"name": options.Name, "version": options.Version, "health": "/health", "logs": "/logs",
			})
			return
		}
		serveHTML(w, dashboardHTML, "dashboard")
	}
}

// Status serves GET /api/status with aggregated dashboard data.
func Status(options StatusOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			options.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		resp := map[string]any{
			"version":        options.Version,
			"uptime_seconds": int(time.Since(options.StartedAt).Seconds()),
			"pid":            os.Getpid(),
			"platform":       runtime.GOOS + "/" + runtime.GOARCH,
		}

		logEntries, logCapacity := options.Logs()
		buffers := map[string]any{"console_entries": logEntries, "console_capacity": logCapacity}

		if options.Capture != nil {
			snap := options.Capture.GetHealthSnapshot()

			resp["extension_connected"] = options.Capture.IsExtensionConnected()
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

			resp["recent_commands"] = buildRecentCommands(options.Capture.GetHTTPDebugLog())
		} else {
			resp["extension_connected"] = false
			resp["pilot_enabled"] = false
		}

		resp["buffers"] = buffers

		// Terminal server status
		termPort, sessions, sessionIDs := options.Terminal()
		termInfo := map[string]any{
			"port":     termPort,
			"running":  termPort > 0,
			"sessions": sessions,
		}
		if sessionIDs != nil {
			termInfo["session_ids"] = sessionIDs
		}
		resp["terminal"] = termInfo
		resp["listen_port"] = options.ListenPort()

		if audit := options.Audit(); audit != nil {
			resp["audit"] = audit
		}

		options.JSONResponse(w, http.StatusOK, resp)
	}
}

func serveHTML(w http.ResponseWriter, content []byte, name string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(content); err != nil {
		diag.Printf("[Kaboom] failed to write %s response: %v\n", name, err)
	}
}

func page(content []byte, name string, respond JSONResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		serveHTML(w, content, name)
	}
}

func Diagnostics(respond JSONResponse) http.HandlerFunc {
	return page(diagnosticsHTML, "diagnostics", respond)
}
func Logs(respond JSONResponse) http.HandlerFunc { return page(logsHTML, "logs", respond) }
func Setup(respond JSONResponse) http.HandlerFunc {
	return page(setupHTML, "setup", respond)
}
func Docs(respond JSONResponse) http.HandlerFunc { return page(docsHTML, "docs", respond) }

type recentCommand struct {
	Timestamp  time.Time `json:"timestamp"`
	Tool       string    `json:"tool"`
	Params     string    `json:"params"`
	Status     int       `json:"status"`
	DurationMs int64     `json:"duration_ms"`
}

func buildRecentCommands(entries []capture.HTTPDebugEntry) []recentCommand {
	var result []recentCommand
	for _, entry := range entries {
		if entry.Timestamp.IsZero() {
			continue
		}
		tool, params := parseMCPCommand(entry.RequestBody)
		result = append(result, recentCommand{
			Timestamp:  entry.Timestamp,
			Tool:       tool,
			Params:     params,
			Status:     entry.ResponseStatus,
			DurationMs: entry.DurationMs,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	if len(result) > 15 {
		result = result[:15]
	}
	return result
}

func parseMCPCommand(body string) (string, string) {
	if body == "" {
		return "unknown", ""
	}
	var request struct {
		Method string `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		return "unknown", ""
	}
	if request.Method != "tools/call" || request.Params.Name == "" {
		return request.Method, ""
	}

	tool := request.Params.Name
	args := request.Params.Arguments
	if len(args) == 0 {
		return tool, ""
	}
	var parts []string
	switch tool {
	case "observe":
		appendDashboardParam(&parts, args, "what")
	case "interact":
		appendDashboardParam(&parts, args, "what")
		appendDashboardParam(&parts, args, "url")
		appendDashboardParam(&parts, args, "selector")
	case "analyze":
		appendDashboardParam(&parts, args, "what")
		appendDashboardParam(&parts, args, "selector")
	case "generate":
		appendDashboardParam(&parts, args, "what")
	case "configure":
		appendDashboardParam(&parts, args, "what")
		appendDashboardParam(&parts, args, "buffer")
		appendDashboardParam(&parts, args, "noise_action")
	default:
		for key, value := range args {
			if text, ok := value.(string); ok && text != "" {
				parts = append(parts, key+"="+truncateDashboardParam(text))
				break
			}
		}
	}
	return tool, strings.Join(parts, " ")
}

func appendDashboardParam(parts *[]string, args map[string]any, key string) {
	value, ok := args[key]
	if !ok {
		return
	}
	text, ok := value.(string)
	if ok && text != "" {
		*parts = append(*parts, key+"="+truncateDashboardParam(text))
	}
}

func truncateDashboardParam(value string) string {
	if len(value) > 40 {
		return value[:37] + "..."
	}
	return value
}
