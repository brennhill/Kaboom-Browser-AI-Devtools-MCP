// Purpose: Defines the Server struct and startup wiring for log, push, and annotation subsystems.
// Why: Centralizes top-level server state while detailed persistence and logging mechanics live in focused modules.

package main

import (
	_ "embed"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/ports"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/healthreader"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/warningqueue"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	annotationruntime "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation/runtime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/transcript"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/http/ci"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/http/dashboard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/http/insecureproxy"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/http/openapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/http/operational"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/http/testpages"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/daemonlog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/toolruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/hooks/tokensavings"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/ingestion/http"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation/adapters/http"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/activecodebase"
	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/daemon/recovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/doctor/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/doctor/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/doctor/support"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/listenport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/adapters/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/adapters/telemetryapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/perftrace"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/state/paths"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/util"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push/adapters/http"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/sync/runtime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters/intent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters/sessionrelay"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters/status"
	terminalsupervisor "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters/supervisor"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/clientreg/http"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/commandhealth"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcpserver/http"
	uploadapi "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

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
	stateRecovery toolruntime.StateDiagnostics
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
	var stateRecovery toolruntime.StateDiagnostics = statediag.NewCollector()
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

// toolRuntimeState projects the daemon stores the five-tool runtime reads. It is
// the only place the daemon hands its internals to the MCP runtime, so the
// runtime never depends on the whole Server and can be composed elsewhere.
func (s *Server) toolRuntimeState() toolruntime.ServerState {
	return toolruntime.ServerState{
		Version:            version,
		Runtime:            s.runtime,
		SessionProjectPath: s.sessionProjectPath,
		Logs:               s.logs,
		Warnings:           s.warnings,
		Incidents:          s.incidents,
		PushInbox:          s.pushInbox,
		ActiveCodebase:     s.activeCodebase,
		AnnotationRuntime:  s.annotationRuntime,
		ListenPort:         s.listenPort,
		UploadSecurity:     s.uploadSecurity,
		TerminalStatus:     s.terminalStatus,
		IntentStore:        s.intentStore,
		StateRecovery:      s.stateRecovery,
	}
}

// Close gracefully shuts down the server, draining the async log writer.
func (s *Server) Close() {
	s.logs.Shutdown(asyncLoggerDrainTimeout)
	s.annotationRuntime.Close()
}

//go:embed openapi.json
var openapiJSON []byte

func setupHTTPRoutes(server *Server, captured *capture.Capture) (*http.ServeMux, *toolruntime.ToolHandler) {
	var mediaResults ports.ResultSink
	if captured != nil {
		mediaResults = captured.Commands()
	}
	server.mediaHTTP = mediaapi.New(mediaResults, server.annotationRuntime.Store(), server.pushRouter)
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
		Telemetry: captured.Telemetry(), Commands: captured.Commands(), Recordings: captured.Recordings(),
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
	clientapi.Register(mux, captured.Clients(), maxPostBodySize)
	mux.HandleFunc("/recordings/save", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		screenrec.HandleSave(w, r, captured.Commands(), server.stateRecovery)
	})))
	mux.HandleFunc("/recordings/storage", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleRecordingStorage)))
	mux.HandleFunc("/recordings/reveal", httpguard.CORS(httpguard.ExtensionOnly(screenrec.HandleReveal)))
	mux.HandleFunc("/telemetry", httpguard.CORS(telemetryapi.Handler(server.logs, telemetryapi.Sources{
		Telemetry: captured.Telemetry(), Performance: captured.Performance(), ExtensionLogs: captured.ExtensionLogs(),
	})))
	mux.HandleFunc("/snapshot", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Snapshot(server.logs, captured))))
	mux.HandleFunc("/clear", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Clear(server.logs, toolruntime.NewRuntimeResetter(captured)))))
	mux.HandleFunc("/test-boundary", httpguard.CORS(httpguard.ExtensionOnly(ciapi.TestBoundary(captured))))
}

func newSyncHandler(captured *capture.Capture) *syncruntime.Handler {
	return syncruntime.NewHandler(syncruntime.Dependencies{
		Runtime:        captured.Extension(),
		Commands:       captured.Commands(),
		Lifecycle:      captured.Lifecycle(),
		FeatureUsage:   captured.FeatureUsage(),
		ExtensionLogs:  captured.ExtensionLogs(),
		DiagnosticLogs: captured.DiagnosticLogs(),
		Recorder:       newExchangeRecorder(),
	})
}

// newExchangeRecorder enables command-exchange recording when
// KABOOM_SYNC_TRANSCRIPT names a file. Off by default and nil in production;
// one live run with it set produces the transcript the replay extension needs
// to run the connected UAT categories without a browser.
func newExchangeRecorder() syncruntime.ExchangeRecorder {
	path := strings.TrimSpace(os.Getenv("KABOOM_SYNC_TRANSCRIPT"))
	if path == "" {
		// EXPECTED_ABSENCE: recording is opt-in, so an unset variable is the
		// normal production configuration rather than a missing setting.
		return nil
	}
	diag.Printf("kaboom: recording sync command exchanges to %s\n", path)
	return synctranscript.NewFileRecorder(path)
}

func registerUploadRoutes(mux *http.ServeMux, server *Server) {
	handlers := uploadapi.NewHandlers(server.uploadSecurity, server.uploadAutomation, httpapi.JSON)
	mux.HandleFunc("/api/file/read", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleFileRead)))
	mux.HandleFunc("/api/file/dialog/inject", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleFileDialogInject)))
	mux.HandleFunc("/api/form/submit", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleFormSubmit)))
	mux.HandleFunc("/api/os-automation/inject", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleOSAutomation)))
	mux.HandleFunc("/api/os-automation/dismiss", httpguard.CORS(httpguard.ExtensionOnly(handlers.HandleOSAutomationDismiss)))
}

func registerCoreRoutes(mux *http.ServeMux, server *Server, captured *capture.Capture) *toolruntime.ToolHandler {
	mux.HandleFunc("/openapi.json", httpguard.CORS(httpapi.OpenAPI(openapiJSON)))

	mcpHandler := toolruntime.NewToolHandler(server.toolRuntimeState(), captured)
	mux.HandleFunc("/mcp", httpguard.CORS(newMCPHTTPHandler(mcpHandler).ServeHTTP))
	operations := operationalapi.New(operationalapi.Options{
		Logs:      server.logs,
		Owners:    healthOwnersOf(captured),
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
		UsageTracker:    mcpHandler.UsageTracker,
		MaxPostBodySize: maxPostBodySize,
	})

	mux.HandleFunc("/api/status", httpguard.CORS(dashboard.Status(dashboard.StatusOptions{
		Version: version, StartedAt: server.runtime.StartedAt(), Owners: healthOwnersOf(captured), JSONResponse: httpapi.JSON,
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
		Audit:      mcpHandler.AuditInfo,
	})))
	mux.HandleFunc("/health", httpguard.CORS(operations.ServeHealth))

	proxyHandler := insecureproxy.New(extensionOf(captured), httpapi.JSON)
	mux.HandleFunc("/insecure-proxy", httpguard.CORS(proxyHandler.ServeHTTP))
	mux.HandleFunc("/doctor", httpguard.CORS(func(w http.ResponseWriter, _ *http.Request) {
		var extraChecks []health.DoctorCheck
		extraChecks = doctorsupport.Checks(mcpHandler.StateRecovery(), server.incidents)
		health.HandleDoctorHTTP(w, healthOwnersOf(captured), version, extraChecks...)
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

func newMCPHTTPHandler(handler *toolruntime.ToolHandler) *mcphttp.Handler {
	return mcphttp.New(mcphttp.Config{
		Version:        version,
		MaxBodySize:    maxPostBodySize,
		HandleRequest:  handler.HandleRequest,
		DiagnosticLogs: healthOwnersOf(handler.Capture()).DiagnosticLogs,
	})
}

// healthOwnersOf returns the health/doctor state owners, or the zero value
// (degraded composition) without a capture.
func healthOwnersOf(captured *capture.Capture) healthreader.Owners {
	if captured == nil {
		return healthreader.Owners{}
	}
	return captured.HealthOwners()
}

// extensionOf returns the extension connection owner, or nil without a capture.
func extensionOf(captured *capture.Capture) *syncruntime.Runtime {
	if captured == nil {
		return nil
	}
	return captured.Extension()
}
