// Purpose: Registers all HTTP routes (/health, /mcp, /telemetry, /shutdown, etc.) and wires handlers to the server mux.
// Why: Centralizes route registration and client-route wiring so endpoint discovery stays simple.

package main

import (
	"net/http"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testpages"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tracking"
)

// setupHTTPRoutes configures the HTTP routes (extracted for reuse).
// Returns both the mux and the MCPHandler so the caller can wire shutdown.
func setupHTTPRoutes(server *Server, cap *capture.Store) (*http.ServeMux, *MCPHandler) {
	mux := http.NewServeMux()

	if cap != nil {
		registerCaptureRoutes(mux, server, cap)
	}

	registerUploadRoutes(mux, server)
	mcpHandler := registerCoreRoutes(mux, server, cap)

	return mux, mcpHandler
}

// registerCaptureRoutes adds capture-dependent routes to the mux.
// NOT MCP — These are extension-to-daemon HTTP endpoints for telemetry ingestion
// and internal state management. AI agents use the 5 MCP tools instead.
func registerCaptureRoutes(mux *http.ServeMux, server *Server, cap *capture.Store) {
	// NOT MCP — Extension telemetry ingestion (extension → daemon data pipeline)
	mux.HandleFunc("/websocket-events", corsMiddleware(extensionOnly(cap.HandleWebSocketEvents)))
	mux.HandleFunc("/websocket-status", corsMiddleware(extensionOnly(cap.HandleWebSocketStatus)))
	mux.HandleFunc("/network-bodies", corsMiddleware(extensionOnly(cap.HandleNetworkBodies)))
	mux.HandleFunc("/network-waterfall", corsMiddleware(extensionOnly(cap.HandleNetworkWaterfall)))
	mux.HandleFunc("/query-result", corsMiddleware(extensionOnly(cap.HandleQueryResult)))
	mux.HandleFunc("/enhanced-actions", corsMiddleware(extensionOnly(cap.HandleEnhancedActions)))
	mux.HandleFunc("/performance-snapshots", corsMiddleware(extensionOnly(cap.HandlePerformanceSnapshots)))

	// NOT MCP — Unified sync endpoint (extension polls this instead of individual routes above)
	mux.HandleFunc("/sync", corsMiddleware(extensionOnly(cap.HandleSync)))

	// NOT MCP — Multi-client registry (extension bookkeeping, not AI-facing)
	registerClientRegistryRoutes(mux, cap)

	// NOT MCP — Video recording binary upload (extension → daemon file storage)
	mux.HandleFunc("/recordings/save", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleVideoRecordingSave(w, r, cap)
	})))

	// NOT MCP — Recording storage management (extension UI)
	mux.HandleFunc("/recordings/storage", corsMiddleware(extensionOnly(cap.HandleRecordingStorage)))

	// NOT MCP — OS file manager integration (opens Finder/Explorer)
	mux.HandleFunc("/recordings/reveal", corsMiddleware(extensionOnly(handleRevealRecording)))

	// NOT MCP — Unified telemetry read (extension and legacy HTTP clients)
	mux.HandleFunc("/telemetry", corsMiddleware(handleTelemetry(server, cap)))

	// NOT MCP — CI infrastructure (test harness boundaries, not AI-facing)
	mux.HandleFunc("/snapshot", corsMiddleware(extensionOnly(handleSnapshot(server, cap))))
	mux.HandleFunc("/clear", corsMiddleware(extensionOnly(handleClear(server, cap))))
	mux.HandleFunc("/test-boundary", corsMiddleware(extensionOnly(handleTestBoundary(cap))))
}

// registerUploadRoutes adds upload automation endpoints to the mux.
// NOT MCP — These are extension-to-daemon escalation stages for file upload automation.
// AI agents use interact(action: "upload") via MCP instead.
// Stages 1-3 are always available; Stage 4 requires --enable-os-upload-automation.
func registerUploadRoutes(mux *http.ServeMux, server *Server) {
	// NOT MCP — File read metadata (upload escalation stage 1, always available)
	mux.HandleFunc("/api/file/read", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleFileRead(w, r)
	})))
	// NOT MCP — File dialog injection (upload escalation stage 2, always available)
	mux.HandleFunc("/api/file/dialog/inject", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleFileDialogInject(w, r)
	})))
	// NOT MCP — Form submit helper (upload escalation stage 3, always available)
	mux.HandleFunc("/api/form/submit", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleFormSubmit(w, r)
	})))
	// NOT MCP — OS-level file dialog automation (upload escalation stage 4, requires --enable-os-upload-automation)
	mux.HandleFunc("/api/os-automation/inject", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleOSAutomation(w, r, osUploadAutomationFlag)
	})))
	// NOT MCP — Dismiss dangling file dialog via Escape key (cleanup after failed Stage 4)
	mux.HandleFunc("/api/os-automation/dismiss", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleOSAutomationDismiss(w, r, osUploadAutomationFlag)
	})))
}

// registerCoreRoutes adds non-capture-dependent routes to the mux.
// Returns the MCPHandler so the caller can wire lifecycle (shutdown, etc.).
func registerCoreRoutes(mux *http.ServeMux, server *Server, cap *capture.Store) *MCPHandler {
	// NOT MCP — OpenAPI spec for HTTP API documentation
	mux.HandleFunc("/openapi.json", corsMiddleware(handleOpenAPI))

	// MCP — The single MCP JSON-RPC endpoint. All AI agent tool calls go through here.
	mcp := NewToolHandler(server, cap)
	mux.HandleFunc("/mcp", corsMiddleware(mcp.HandleHTTP))

	// NOT MCP — Dashboard status API (JSON feed for the HTML dashboard)
	mux.HandleFunc("/api/status", corsMiddleware(handleStatusAPI(server, cap, mcp)))

	// NOT MCP — Health check for extension and monitoring (MCP uses configure(action: "health"))
	mux.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleHealth(w, r, cap)
	}))

	// NOT MCP — Last-resort altered-environment proxy for CSP-locked debugging sessions.
	mux.HandleFunc("/insecure-proxy", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleInsecureProxy(w, r, cap)
	}))

	// NOT MCP — Doctor preflight check (aggregated readiness status)
	mux.HandleFunc("/doctor", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		health.HandleDoctorHTTP(w, cap, version)
	}))

	// NOT MCP — Token savings tracking from hook scripts (POST from output-compression-hook.sh)
	mux.HandleFunc("/api/token-savings", corsMiddleware(tracking.HandleRecordTokenSavings(server.tokenTracker)))

	// NOT MCP — Debug: telemetry usage counter inspection and beacon flush.
	// Gated behind KABOOM_DEBUG=1 to prevent accidental exposure in production.
	if debugEndpointsEnabled() {
		mux.HandleFunc("/debug/usage", corsMiddleware(handleDebugUsage(mcp)))
		mux.HandleFunc("/debug/beacon-flush", corsMiddleware(handleDebugBeaconFlush(mcp)))
	}

	// NOT MCP — Graceful shutdown (use CLI --stop flag, not MCP)
	mux.HandleFunc("/shutdown", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleShutdown(w, r)
	})))

	// NOT MCP — Debug diagnostics: HTML for browsers, JSON for programmatic access
	mux.HandleFunc("/diagnostics", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json") {
			serveEmbeddedHTML(w, r, diagnosticsHTML, "diagnostics")
			return
		}
		server.handleDiagnostics(w, r, cap)
	}))
	mux.HandleFunc("/diagnostics.json", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleDiagnostics(w, r, cap)
	}))

	// NOT MCP — Log ingestion from extension (MCP reads logs via observe(what: "logs"))
	mux.HandleFunc("/logs", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleLogs(w, r)
	})))

	// NOT MCP — HTML pages for human navigation
	mux.HandleFunc("/logs.html", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, logsHTML, "logs")
	}))
	mux.HandleFunc("/setup", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, setupHTML, "setup")
	}))
	mux.HandleFunc("/docs", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, docsHTML, "docs")
	}))

	// NOT MCP — WebSocket echo server for test harness (must be registered before /tests/ subtree).
	// corsMiddleware sets headers on http.ResponseWriter pre-hijack; those headers are not included
	// in the manually-written 101 response (intentional — WS upgrade bypasses HTTP CORS).
	mux.HandleFunc("/tests/ws", corsMiddleware(testpages.HandlerWS))
	// NOT MCP — Embedded test/demo pages for self-testing
	mux.HandleFunc("/tests/", corsMiddleware(testpages.Handler()))

	// NOT MCP — Screenshot binary upload from extension (MCP reads via observe(what: "screenshot"))
	mux.HandleFunc("/screenshots", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleScreenshot(w, r, cap)
	})))

	// NOT MCP — Draw mode completion callback from extension (MCP uses analyze(what: "annotations"))
	mux.HandleFunc("/draw-mode/complete", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleDrawModeComplete(w, r, cap)
	})))

	// NOT MCP — Push pipeline endpoints (extension → daemon → AI client)
	mux.HandleFunc("/push/screenshot", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handlePushScreenshot(w, r)
	})))
	mux.HandleFunc("/push/message", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handlePushMessage(w, r)
	})))
	mux.HandleFunc("/push/capabilities", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handlePushCapabilities(w, r)
	})))
	// Bridge push relay: internal endpoint for the bridge process to drain push events.
	// No extensionOnly — called by the bridge process, not the browser extension.
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
	mux.HandleFunc("/config/active-codebase", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleActiveCodebase(w, r, server)
	})))

	// NOT MCP — HTML dashboard (browser) with JSON fallback (Accept: application/json)
	mux.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleDashboard(w, r)
	}))

	return mcp
}
