// Purpose: Registers all HTTP routes (/health, /mcp, /telemetry, /shutdown, etc.) and wires handlers to the server mux.
// Why: Centralizes route registration and client-route wiring so endpoint discovery stays simple.

package main

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/ciapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/insecureproxy"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testpages"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tracking"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/httpapi"
)

//go:embed openapi.json
var openapiJSON []byte

func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(openapiJSON); err != nil {
		diag.Printf("[kaboom] failed to write /openapi.json response: %v\n", err)
	}
}

func handleTelemetry(server *Server, cap *capture.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		telType := r.URL.Query().Get("type")
		if telType == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{
				"error": "Missing required 'type' parameter",
				"hint":  "Valid types: logs, network_waterfall, network_bodies, websocket_events, actions, performance_snapshots, extension_logs, websocket_status",
			})
			return
		}

		limit := 0
		if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
			if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		var result any
		var count int
		switch telType {
		case "logs":
			entries := server.logs.Entries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "network_waterfall":
			entries := cap.GetNetworkWaterfallEntries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "network_bodies":
			entries := cap.GetNetworkBodies()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "websocket_events":
			entries := cap.GetWebSocketEvents(capture.WebSocketEventFilter{})
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "actions":
			entries := cap.GetAllEnhancedActions()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "performance_snapshots":
			entries := cap.GetPerformanceSnapshots()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "extension_logs":
			entries := cap.GetExtensionLogs()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "websocket_status":
			status := cap.GetWebSocketStatus(capture.WebSocketStatusFilter{})
			jsonResponse(w, http.StatusOK, map[string]any{
				"type": telType, "connections": status.Connections,
				"closed": status.Closed, "count": len(status.Connections),
			})
			return
		default:
			jsonResponse(w, http.StatusBadRequest, map[string]string{
				"error": "Unknown telemetry type: " + telType,
				"hint":  "Valid types: logs, network_waterfall, network_bodies, websocket_events, actions, performance_snapshots, extension_logs, websocket_status",
			})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"type": telType, "items": result, "count": count,
		})
	}
}

// setupHTTPRoutes configures the HTTP routes (extracted for reuse).
// Returns both the mux and the MCPHandler so the caller can wire shutdown.
func setupHTTPRoutes(server *Server, cap *capture.Store) (*http.ServeMux, *MCPHandler) {
	mux := http.NewServeMux()

	if cap != nil {
		registerCaptureRoutes(mux, server, cap)
	}

	registerUploadRoutes(mux)
	mcpHandler := registerCoreRoutes(mux, server, cap)

	return mux, mcpHandler
}

// registerCaptureRoutes adds capture-dependent routes to the mux.
// NOT MCP — These are extension-to-daemon HTTP endpoints for telemetry ingestion
// and internal state management. AI agents use the 5 MCP tools instead.
func registerCaptureRoutes(mux *http.ServeMux, server *Server, cap *capture.Store) {
	// NOT MCP — Extension telemetry ingestion (extension → daemon data pipeline)
	mux.HandleFunc("/websocket-events", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleWebSocketEvents)))
	mux.HandleFunc("/websocket-status", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleWebSocketStatus)))
	mux.HandleFunc("/network-bodies", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleNetworkBodies)))
	mux.HandleFunc("/network-waterfall", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleNetworkWaterfall)))
	mux.HandleFunc("/query-result", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleQueryResult)))
	mux.HandleFunc("/enhanced-actions", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleEnhancedActions)))
	mux.HandleFunc("/performance-snapshots", httpguard.CORS(httpguard.ExtensionOnly(cap.HandlePerformanceSnapshots)))

	// NOT MCP — Unified sync endpoint (extension polls this instead of individual routes above)
	mux.HandleFunc("/sync", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleSync)))

	// NOT MCP — Multi-client registry (extension bookkeeping, not AI-facing)
	registerClientRegistryRoutes(mux, cap)

	// NOT MCP — Video recording binary upload (extension → daemon file storage)
	mux.HandleFunc("/recordings/save", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		screenrec.HandleSave(w, r, cap)
	})))

	// NOT MCP — Recording storage management (extension UI)
	mux.HandleFunc("/recordings/storage", httpguard.CORS(httpguard.ExtensionOnly(cap.HandleRecordingStorage)))

	// NOT MCP — OS file manager integration (opens Finder/Explorer)
	mux.HandleFunc("/recordings/reveal", httpguard.CORS(httpguard.ExtensionOnly(screenrec.HandleReveal)))

	// NOT MCP — Unified telemetry read (extension and legacy HTTP clients)
	mux.HandleFunc("/telemetry", httpguard.CORS(handleTelemetry(server, cap)))

	// NOT MCP — CI infrastructure (test harness boundaries, not AI-facing)
	mux.HandleFunc("/snapshot", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Snapshot(server.logs, cap))))
	mux.HandleFunc("/clear", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Clear(server.logs, cap))))
	mux.HandleFunc("/test-boundary", httpguard.CORS(httpguard.ExtensionOnly(ciapi.TestBoundary(cap))))
}

func resolveClientRegistry(cap *capture.Store, w http.ResponseWriter) (capture.ClientRegistry, bool) {
	registry := cap.GetClientRegistry()
	if registry == nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "client_registry_unavailable"})
		return nil, false
	}
	return registry, true
}

func registerClientRegistryRoutes(mux *http.ServeMux, cap *capture.Store) {
	mux.HandleFunc("/clients", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientsList(w, r, cap)
	})))
	mux.HandleFunc("/clients/", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientByID(w, r, cap)
	})))
}

func handleClientsList(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	registry, ok := resolveClientRegistry(cap, w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, map[string]any{
			"clients": registry.List(),
			"count":   registry.Count(),
		})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
		var body struct {
			CWD string `json:"cwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"result": registry.Register(body.CWD)})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func handleClientByID(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	registry, ok := resolveClientRegistry(cap, w)
	if !ok {
		return
	}
	clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
	if clientID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		client := registry.Get(clientID)
		if client == nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		jsonResponse(w, http.StatusOK, client)
	case http.MethodDelete:
		if !registry.Unregister(clientID) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"unregistered": true})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// registerUploadRoutes adds upload automation endpoints to the mux.
// NOT MCP — These are extension-to-daemon escalation stages for file upload automation.
// AI agents use interact(action: "upload") via MCP instead.
// Stages 1-3 are always available; Stage 4 requires --enable-os-upload-automation.
func registerUploadRoutes(mux *http.ServeMux) {
	// NOT MCP — File read metadata (upload escalation stage 1, always available)
	mux.HandleFunc("/api/file/read", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		httpapi.HandleFileReadHTTP(w, r, uploadSecurityConfig, jsonResponse)
	})))
	// NOT MCP — File dialog injection (upload escalation stage 2, always available)
	mux.HandleFunc("/api/file/dialog/inject", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		httpapi.HandleFileDialogInjectHTTP(w, r, uploadSecurityConfig, jsonResponse)
	})))
	// NOT MCP — Form submit helper (upload escalation stage 3, always available)
	mux.HandleFunc("/api/form/submit", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		httpapi.HandleFormSubmitHTTP(w, r, uploadSecurityConfig, jsonResponse)
	})))
	// NOT MCP — OS-level file dialog automation (upload escalation stage 4, requires --enable-os-upload-automation)
	mux.HandleFunc("/api/os-automation/inject", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		httpapi.HandleOSAutomationHTTP(w, r, osUploadAutomationFlag, uploadSecurityConfig, jsonResponse)
	})))
	// NOT MCP — Dismiss dangling file dialog via Escape key (cleanup after failed Stage 4)
	mux.HandleFunc("/api/os-automation/dismiss", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		httpapi.HandleOSAutomationDismissHTTP(w, r, osUploadAutomationFlag, jsonResponse)
	})))
}

// registerCoreRoutes adds non-capture-dependent routes to the mux.
// Returns the MCPHandler so the caller can wire lifecycle (shutdown, etc.).
func registerCoreRoutes(mux *http.ServeMux, server *Server, cap *capture.Store) *MCPHandler {
	// NOT MCP — OpenAPI spec for HTTP API documentation
	mux.HandleFunc("/openapi.json", httpguard.CORS(handleOpenAPI))

	// MCP — The single MCP JSON-RPC endpoint. All AI agent tool calls go through here.
	mcp := NewToolHandler(server, cap)
	mux.HandleFunc("/mcp", httpguard.CORS(mcp.HandleHTTP))

	// NOT MCP — Dashboard status API (JSON feed for the HTML dashboard)
	mux.HandleFunc("/api/status", httpguard.CORS(handleStatusAPI(server, cap, mcp)))

	// NOT MCP — Health check for extension and monitoring (MCP uses configure(action: "health"))
	mux.HandleFunc("/health", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		server.handleHealth(w, r, cap)
	}))

	// NOT MCP — Last-resort altered-environment proxy for CSP-locked debugging sessions.
	proxyHandler := insecureproxy.New(cap, jsonResponse)
	mux.HandleFunc("/insecure-proxy", httpguard.CORS(proxyHandler.ServeHTTP))

	// NOT MCP — Doctor preflight check (aggregated readiness status)
	mux.HandleFunc("/doctor", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		health.HandleDoctorHTTP(w, cap, version)
	}))

	// NOT MCP — Token savings tracking from hook scripts (POST from output-compression-hook.sh)
	mux.HandleFunc("/api/token-savings", httpguard.CORS(tracking.HandleRecordTokenSavings(server.tokenTracker)))

	// NOT MCP — Debug: telemetry usage counter inspection and beacon flush.
	// Gated behind KABOOM_DEBUG=1 to prevent accidental exposure in production.
	if debugEndpointsEnabled() {
		mux.HandleFunc("/debug/usage", httpguard.CORS(handleDebugUsage(mcp)))
		mux.HandleFunc("/debug/beacon-flush", httpguard.CORS(handleDebugBeaconFlush(mcp)))
	}

	// NOT MCP — Graceful shutdown (use CLI --stop flag, not MCP)
	mux.HandleFunc("/shutdown", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleShutdown(w, r)
	})))

	// NOT MCP — Debug diagnostics: HTML for browsers, JSON for programmatic access
	mux.HandleFunc("/diagnostics", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json") {
			serveEmbeddedHTML(w, r, diagnosticsHTML, "diagnostics")
			return
		}
		server.handleDiagnostics(w, r, cap)
	}))
	mux.HandleFunc("/diagnostics.json", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		server.handleDiagnostics(w, r, cap)
	}))

	// NOT MCP — Log ingestion from extension (MCP reads logs via observe(what: "logs"))
	mux.HandleFunc("/logs", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleLogs(w, r)
	})))

	// NOT MCP — HTML pages for human navigation
	mux.HandleFunc("/logs.html", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, logsHTML, "logs")
	}))
	mux.HandleFunc("/setup", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, setupHTML, "setup")
	}))
	mux.HandleFunc("/docs", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, docsHTML, "docs")
	}))

	// NOT MCP — WebSocket echo server for test harness (must be registered before /tests/ subtree).
	// httpguard.CORS sets headers on http.ResponseWriter pre-hijack; those headers are not included
	// in the manually-written 101 response (intentional — WS upgrade bypasses HTTP CORS).
	mux.HandleFunc("/tests/ws", httpguard.CORS(testpages.HandlerWS))
	// NOT MCP — Embedded test/demo pages for self-testing
	mux.HandleFunc("/tests/", httpguard.CORS(testpages.Handler()))

	// NOT MCP — Screenshot binary upload from extension (MCP reads via observe(what: "screenshot"))
	mux.HandleFunc("/screenshots", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleScreenshot(w, r, cap)
	})))

	// NOT MCP — Draw mode completion callback from extension (MCP uses analyze(what: "annotations"))
	mux.HandleFunc("/draw-mode/complete", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleDrawModeComplete(w, r, cap)
	})))

	// NOT MCP — Push pipeline endpoints (extension → daemon → AI client)
	mux.HandleFunc("/push/screenshot", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handlePushScreenshot(w, r)
	})))
	mux.HandleFunc("/push/message", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handlePushMessage(w, r)
	})))
	mux.HandleFunc("/push/capabilities", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handlePushCapabilities(w, r)
	})))
	// Bridge push relay: internal endpoint for the bridge process to drain push events.
	// No httpguard.ExtensionOnly — called by the bridge process, not the browser extension.
	// Token-authenticated when pushDrainToken is configured.
	mux.HandleFunc("/push/drain", func(w http.ResponseWriter, r *http.Request) {
		if server.pushDrainToken != "" {
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) || auth[len(prefix):] != server.pushDrainToken {
				jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		server.handlePushDrain(w, r)
	})

	// NOT MCP — Active codebase GET/PUT — extension reads/writes the default terminal CWD.
	mux.HandleFunc("/config/active-codebase", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleActiveCodebase(w, r, server)
	})))

	// NOT MCP — HTML dashboard (browser) with JSON fallback (Accept: application/json)
	mux.HandleFunc("/", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		server.handleDashboard(w, r)
	}))

	return mcp
}
