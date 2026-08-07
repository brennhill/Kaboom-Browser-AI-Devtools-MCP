// Purpose: Defines the Server struct and startup wiring for log, push, and annotation subsystems.
// Why: Centralizes top-level server state while detailed persistence and logging mechanics live in focused modules.

package main

import (
	_ "embed"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/warningqueue"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/ciapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/clientapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonrecovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/dashboard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/doctorsupport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/insecureproxy"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcphttp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/operationalapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/pushapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/telemetryapi"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/status"
	terminalsupervisor "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/supervisor"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testpages"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/activecodebase"
	annotationruntime "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation/runtime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/httpingest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/resetter"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/listenport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/perftrace"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tracking"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	uploadapi "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

type stateRecoveryDiagnostics interface {
	statediag.Reporter
	statediag.Resolver
	Snapshot() []statediag.Diagnostic
	Stats() statediag.CollectorStats
}

// Server holds the server state.
type Server struct {
	runtime    *appruntime.Runtime
	listenPort *listenport.Store
	mu         sync.RWMutex
	// sessionProjectPath is resolved once at server construction so handlers do
	// not independently bind persistence to a changing process working directory.
	sessionProjectPath string

	// Log subsystem — owns entries, TTL rotation, async channel, file persistence.
	logs *logstore.Store

	// One-shot warnings surfaced via MCP tool responses.
	warnings *warningqueue.Queue

	annotationRuntime *annotationruntime.Owner

	// Push delivery pipeline
	pushInbox  *push.PushInbox
	pushRouter *push.Router
	pushHTTP   *pushapi.Handler
	mediaHTTP  *mediaapi.Handler

	// Terminal PTY session manager
	ptyManager  *pty.Manager
	ptyRelays   *sessionrelay.Map
	intentStore *terminalintent.Store

	terminalStatus *terminalstatus.Store

	// terminalSupervisor watches and auto-restarts the terminal HTTP server.
	// nil if the terminal server never bound (Windows, or bind failure).
	terminalSupervisor *terminalsupervisor.Supervisor

	activeCodebase *activecodebase.Store

	// Token savings tracker for output compression hooks.
	tokenTracker  *tracking.TokenTracker
	stateRecovery stateRecoveryDiagnostics
	incidents     *incident.Store

	// Push drain authentication token. When non-empty, /push/drain requires
	// Authorization: Bearer <token>. Set via --push-drain-token flag.
	pushDrainToken   string
	uploadAutomation bool
	uploadSecurity   *uploadsec.Security
	daemonRecovery   *daemonrecovery.Reclaimer
}

func (s *Server) logLifecycle(event string, port int, fields map[string]any) {
	entry := types.LogEntry{
		"type":      "lifecycle",
		"event":     event,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if port != 0 {
		entry["port"] = port
	}
	for key, value := range fields {
		entry[key] = value
	}
	s.logs.AddEntries([]types.LogEntry{entry})
}

// NewServer creates a new server instance.
func NewServer(logFile string, maxEntries int) (*Server, error) {
	sessionProjectPath, pathErr := os.Getwd()
	if pathErr != nil {
		return nil, fmt.Errorf("resolve session project path: %w", pathErr)
	}
	var stateRecovery stateRecoveryDiagnostics = statediag.NewCollector()
	timelinePath, timelinePathErr := state.InRoot("doctor", "incident-timeline.json")
	if timelinePathErr == nil {
		persistent, loadErr := statediag.NewPersistentCollector(timelinePath)
		stateRecovery = persistent
		if loadErr != nil {
			stateRecovery.Report(statediag.Diagnostic{
				Name: "doctor_timeline_state", Detail: "Saved Doctor incident history was invalid or unreadable; a clean local timeline is active.",
				Fix: "No action is required unless this repeats; check the Kaboom state directory permissions.",
			})
		}
	} else {
		stateRecovery.Report(statediag.Diagnostic{
			Name: "doctor_timeline_state", Detail: "Doctor incident history could not resolve its local state path.",
			Fix: "Check the Kaboom state directory configuration.",
		})
	}

	s := &Server{
		runtime:            appruntime.New(version),
		listenPort:         listenport.New(),
		sessionProjectPath: sessionProjectPath,
		warnings:           warningqueue.New(),
		annotationRuntime:  annotationruntime.New(10 * time.Minute),
		activeCodebase:     activecodebase.New(),
		pushInbox:          push.NewPushInbox(50),
		ptyManager:         pty.NewManager(),
		tokenTracker:       tracking.NewTokenTracker(),
		intentStore:        terminalintent.NewStore(),
		terminalStatus:     terminalstatus.New(),
		stateRecovery:      stateRecovery,
		incidents:          incident.NewStore(100, telemetry.QueueReliability),
	}

	// Create log store with warning callback wired to server
	s.logs = logstore.New(logstore.Config{
		LogFile:       logFile,
		MaxEntries:    maxEntries,
		TelemetryMode: mcptelemetry.ModeAuto,
		AddWarning:    s.warnings.Add,
		Stderrf:       diag.Printf,
	})
	s.daemonRecovery = daemonrecovery.New(daemonrecovery.Config{
		Version: version, Recovery: s.stateRecovery, Incidents: s.incidents,
		LogLifecycle: s.logLifecycle, Diagnosticf: diag.Printf,
	})

	// Initialize push router with capability sync callback
	caps := cmbridge.PushRuntime.Capabilities()
	s.pushRouter = push.NewRouter(s.pushInbox, cmbridge.PushRuntime, cmbridge.PushRuntime, caps)
	s.pushHTTP = pushapi.NewHandler(s.pushRouter, s.pushInbox, cmbridge.PushRuntime, httpapi.JSON, maxPostBodySize)
	cmbridge.PushRuntime.OnCapabilitiesChange(func(newCaps push.ClientCapabilities) {
		s.pushRouter.UpdateCapabilities(newCaps)
	})

	// Start async logger goroutine
	logs := s.logs
	util.SafeGo(logs.RunWorker)

	logstore.PreparePersistence(s.logs, s.warnings.Add)

	return s, nil
}

// Close gracefully shuts down the server, draining the async log writer.
func (s *Server) Close() {
	s.logs.Shutdown(asyncLoggerDrainTimeout)
	s.annotationRuntime.Close()
}

//go:embed openapi.json
var openapiJSON []byte

func setupHTTPRoutes(server *Server, captured *capture.Capture) (*http.ServeMux, *MCPHandler) {
	server.mediaHTTP = mediaapi.New(captured, server.annotationRuntime.Store(), server.pushRouter)
	mux := http.NewServeMux()
	if captured != nil {
		registerCaptureRoutes(mux, server, captured)
	}
	registerUploadRoutes(mux, server)
	registerPerformanceTraceRoutes(mux)
	return mux, registerCoreRoutes(mux, server, captured)
}

func registerPerformanceTraceRoutes(mux *http.ServeMux) {
	dir, err := state.PerformanceTracesDir()
	if err != nil {
		diag.Printf("[Kaboom] performance trace state path unavailable; using process-local temporary storage: %v\n", err)
		dir = filepath.Join(os.TempDir(), "kaboom-performance-traces")
	}
	handler := perftrace.NewHTTPHandler(perftrace.NewManager(dir))
	mux.HandleFunc("/performance-trace/start", httpguard.CORS(httpguard.ExtensionOnly(handler.HandleStart)))
	mux.HandleFunc("/performance-trace/chunk", httpguard.CORS(httpguard.ExtensionOnly(handler.HandleChunk)))
	mux.HandleFunc("/performance-trace/finish", httpguard.CORS(httpguard.ExtensionOnly(handler.HandleFinish)))
	mux.HandleFunc("/performance-trace/abort", httpguard.CORS(httpguard.ExtensionOnly(handler.HandleAbort)))
}

func registerCaptureRoutes(mux *http.ServeMux, server *Server, captured *capture.Capture) {
	captureHTTP := httpingest.New(httpingest.Dependencies{
		Telemetry: captured.Telemetry(), Queries: captured.Queries(), Recordings: captured.Recordings(),
		Performance: captured.Performance(), Circuit: captured.Circuit(),
	})
	mux.HandleFunc("/websocket-events", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleWebSocketEvents)))
	mux.HandleFunc("/websocket-status", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleWebSocketStatus)))
	mux.HandleFunc("/network-bodies", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleNetworkBodies)))
	mux.HandleFunc("/network-waterfall", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleNetworkWaterfall)))
	mux.HandleFunc("/query-result", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleQueryResult)))
	mux.HandleFunc("/enhanced-actions", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleEnhancedActions)))
	mux.HandleFunc("/performance-snapshots", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandlePerformanceSnapshots)))
	mux.HandleFunc("/sync", httpguard.CORS(httpguard.ExtensionOnly(newSyncHandler(captured).HandleSync)))
	clientapi.Register(mux, captured, maxPostBodySize)
	mux.HandleFunc("/recordings/save", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		screenrec.HandleSave(w, r, captured.Queries(), server.stateRecovery)
	})))
	mux.HandleFunc("/recordings/storage", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleRecordingStorage)))
	mux.HandleFunc("/recordings/reveal", httpguard.CORS(httpguard.ExtensionOnly(screenrec.HandleReveal)))
	mux.HandleFunc("/telemetry", httpguard.CORS(telemetryapi.Handler(server.logs, captured)))
	mux.HandleFunc("/snapshot", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Snapshot(server.logs, captured))))
	mux.HandleFunc("/clear", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Clear(server.logs, newRuntimeResetter(captured)))))
	mux.HandleFunc("/test-boundary", httpguard.CORS(httpguard.ExtensionOnly(ciapi.TestBoundary(captured))))
}

func newRuntimeResetter(captured *capture.Capture) *resetter.Resetter {
	return resetter.New(resetter.Dependencies{Extension: captured.Extension(), Telemetry: captured.Telemetry(), Performance: captured.Performance(), ExtensionLogs: captured.ExtensionLogs()})
}

func newSyncHandler(captured *capture.Capture) *syncruntime.Handler {
	return syncruntime.NewHandler(syncruntime.Dependencies{
		Runtime:        captured.Extension(),
		Queries:        captured.Queries(),
		Lifecycle:      captured.Lifecycle(),
		FeatureUsage:   captured.FeatureUsage(),
		ExtensionLogs:  captured.ExtensionLogs(),
		DiagnosticLogs: captured.DiagnosticLogs(),
	})
}

func registerUploadRoutes(mux *http.ServeMux, server *Server) {
	handlers := uploadapi.NewHandlers(server.uploadSecurity, server.uploadAutomation, httpapi.JSON)
	mux.HandleFunc("/api/file/read", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleFileRead)))
	mux.HandleFunc("/api/file/dialog/inject", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleFileDialogInject)))
	mux.HandleFunc("/api/form/submit", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleFormSubmit)))
	mux.HandleFunc("/api/os-automation/inject", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleOSAutomation)))
	mux.HandleFunc("/api/os-automation/dismiss", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleOSAutomationDismiss)))
}

func registerCoreRoutes(mux *http.ServeMux, server *Server, captured *capture.Capture) *MCPHandler {
	mux.HandleFunc("/openapi.json", httpguard.CORS(httpapi.OpenAPI(openapiJSON)))

	mcpHandler := NewToolHandler(server, captured)
	mux.HandleFunc("/mcp", httpguard.CORS(newMCPHTTPHandler(mcpHandler).ServeHTTP))
	operations := operationalapi.New(operationalapi.Options{
		Logs:      server.logs,
		Capture:   captured,
		Version:   version,
		StartedAt: server.runtime.StartedAt(),
		TerminalStatus: func() operationalapi.TerminalStatus {
			status := server.terminalStatus.Snapshot()
			return operationalapi.TerminalStatus{
				Available:      status.Available,
				Port:           status.Port,
				Error:          status.Error,
				BlockedByPID:   status.BlockedByPID,
				BlockedCommand: status.BlockedByCommand,
			}
		},
		AvailableVersion: server.runtime.ReleaseChecker().Available,
		UpgradeInfo: func() *health.UpgradeInfo {
			if server.runtime.Upgrade() == nil {
				return nil
			}
			return health.BuildUpgradeInfo(server.runtime.Upgrade())
		},
		UsageTracker:    func() *telemetry.UsageTracker { return mcpHandler.tools.UsageTracker },
		MaxPostBodySize: maxPostBodySize,
	})

	mux.HandleFunc("/api/status", httpguard.CORS(dashboard.Status(dashboard.StatusOptions{
		Version: version, StartedAt: server.runtime.StartedAt(), Capture: captured, JSONResponse: httpapi.JSON,
		Logs: func() (int, int) {
			return server.logs.EntryCount(), server.logs.MaxEntries()
		},
		Terminal: func() (int, int, []string) {
			port := server.terminalStatus.Port()
			if server.ptyManager == nil {
				return port, 0, nil
			}
			return port, server.ptyManager.Count(), server.ptyManager.List()
		},
		ListenPort: server.listenPort.Get,
		Audit: func() any {
			if mcpHandler.tools.Executor == nil {
				return nil
			}
			handler, ok := mcpHandler.tools.Executor.(*ToolHandler)
			if !ok || handler.healthMetrics == nil {
				return nil
			}
			return handler.healthMetrics.BuildAuditInfo()
		},
	})))
	mux.HandleFunc("/health", httpguard.CORS(operations.ServeHealth))

	proxyHandler := insecureproxy.New(captured, httpapi.JSON)
	mux.HandleFunc("/insecure-proxy", httpguard.CORS(proxyHandler.ServeHTTP))
	mux.HandleFunc("/doctor", httpguard.CORS(func(w http.ResponseWriter, _ *http.Request) {
		var extraChecks []health.DoctorCheck
		if handler, ok := mcpHandler.tools.Executor.(*ToolHandler); ok {
			extraChecks = doctorsupport.Checks(handler.stateRecovery, server.incidents)
		}
		health.HandleDoctorHTTP(w, captured, version, extraChecks...)
	}))
	mux.HandleFunc("/api/token-savings", httpguard.CORS(tracking.HandleRecordTokenSavings(server.tokenTracker)))

	if operationalapi.DebugEndpointsEnabled() {
		mux.HandleFunc("/debug/usage", httpguard.CORS(operations.ServeDebugUsage))
		mux.HandleFunc("/debug/beacon-flush", httpguard.CORS(operations.ServeDebugBeaconFlush))
	}
	mux.HandleFunc("/shutdown", httpguard.CORS(httpguard.ExtensionOnly(operations.ServeShutdown)))
	mux.HandleFunc("/diagnostics", httpguard.CORS(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json") {
			dashboard.Diagnostics(httpapi.JSON)(w, r)
			return
		}
		operations.ServeDiagnostics(w, r)
	}))
	mux.HandleFunc("/logs", httpguard.CORS(httpguard.ExtensionOnly(operations.ServeLogs)))
	mux.HandleFunc("/logs.html", httpguard.CORS(dashboard.Logs(httpapi.JSON)))
	mux.HandleFunc("/setup", httpguard.CORS(dashboard.Setup(httpapi.JSON)))
	mux.HandleFunc("/docs", httpguard.CORS(dashboard.Docs(httpapi.JSON)))

	mux.HandleFunc("/tests/ws", httpguard.CORS(testpages.HandlerWS))
	mux.HandleFunc("/tests/", httpguard.CORS(testpages.Handler()))
	mux.HandleFunc("/screenshots", httpguard.CORS(httpguard.ExtensionOnly(server.mediaHTTP.HandleScreenshot)))
	mux.HandleFunc("/draw-mode/complete", httpguard.CORS(httpguard.ExtensionOnly(server.mediaHTTP.HandleDrawModeComplete)))
	mux.HandleFunc("/push/screenshot", httpguard.CORS(httpguard.ExtensionOnly(server.pushHTTP.HandleScreenshot)))
	mux.HandleFunc("/push/message", httpguard.CORS(httpguard.ExtensionOnly(server.pushHTTP.HandleMessage)))
	mux.HandleFunc("/push/capabilities", httpguard.CORS(httpguard.ExtensionOnly(server.pushHTTP.HandleCapabilities)))
	mux.HandleFunc("/push/drain", func(w http.ResponseWriter, r *http.Request) {
		if server.pushDrainToken != "" {
			const prefix = "Bearer "
			authorization := r.Header.Get("Authorization")
			if !strings.HasPrefix(authorization, prefix) || authorization[len(prefix):] != server.pushDrainToken {
				httpapi.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		server.pushHTTP.HandleDrain(w, r)
	})
	mux.HandleFunc("/config/active-codebase", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleActiveCodebase(w, r, server)
	})))
	mux.HandleFunc("/", httpguard.CORS(dashboard.Root(dashboard.RootOptions{
		Name: identity.MCPServerName, Version: version, JSONResponse: httpapi.JSON,
	})))
	return mcpHandler
}

func newMCPHTTPHandler(handler *MCPHandler) *mcphttp.Handler {
	return mcphttp.New(mcphttp.Config{
		Version:       handler.version,
		MaxBodySize:   maxPostBodySize,
		HandleRequest: handler.HandleRequest,
		Capture: func() *capture.Capture {
			return handler.tools.Capture
		},
	})
}
