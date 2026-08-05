// Purpose: Defines the Server struct and startup wiring for log, push, and annotation subsystems.
// Why: Centralizes top-level server state while detailed persistence and logging mechanics live in focused modules.

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/ciapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/dashboard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/insecureproxy"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcphttp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/operationalapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/pushapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	terminalsupervisor "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/supervisor"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testpages"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/clientstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/httpingest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/perftrace"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tracking"
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
	listenPort int
	mu         sync.RWMutex
	// sessionProjectPath is resolved once at server construction so handlers do
	// not independently bind persistence to a changing process working directory.
	sessionProjectPath string

	// Log subsystem — owns entries, TTL rotation, async channel, file persistence.
	logs *logstore.Store

	// One-shot warnings surfaced via MCP tool responses.
	warningsMu  sync.Mutex
	warnings    []string
	warningSeen map[string]struct{}

	// Annotation store is server-scoped to avoid cross-session contamination.
	annotationStore *annotation.Store

	// Push delivery pipeline
	pushInbox  *push.PushInbox
	pushRouter *push.Router
	pushHTTP   *pushapi.Handler
	mediaHTTP  *mediaapi.Handler

	// Terminal PTY session manager
	ptyManager  *pty.Manager
	ptyRelays   *terminal.Map
	intentStore *terminal.IntentStore

	// Terminal server port (0 = terminal server not running)
	terminalPort int
	// Terminal diagnosability: a bind failure is non-fatal, so /health is the only
	// place the reason can surface (daemon stderr goes to /dev/null when spawned).
	terminalAvailable        bool
	terminalWantedPort       int
	terminalError            string
	terminalBlockedByPID     int
	terminalBlockedByCommand string

	// terminalSupervisor watches and auto-restarts the terminal HTTP server.
	// nil if the terminal server never bound (Windows, or bind failure).
	terminalSupervisor *terminalsupervisor.Supervisor

	// Active codebase path — set via MCP configure(what='store', key='active_codebase')
	// or via the extension options page. Used as default CWD for terminal sessions.
	activeCodebaseMu sync.RWMutex
	activeCodebase   string

	// Token savings tracker for output compression hooks.
	tokenTracker  *tracking.TokenTracker
	stateRecovery stateRecoveryDiagnostics
	incidents     *incident.Store

	// Push drain authentication token. When non-empty, /push/drain requires
	// Authorization: Bearer <token>. Set via --push-drain-token flag.
	pushDrainToken   string
	uploadAutomation bool
	uploadSecurity   *uploadsec.Security
	daemonHost       daemonHost
}

func (s *Server) applyRuntimeConfig(config *serverConfig) {
	s.uploadAutomation = config.uploadAutomation
	s.uploadSecurity = config.uploadSecurity
}

// AddWarning stores a unique one-shot warning for the next tool response.
func (s *Server) AddWarning(message string) {
	if message == "" {
		return
	}
	s.warningsMu.Lock()
	defer s.warningsMu.Unlock()
	if s.warningSeen == nil {
		s.warningSeen = make(map[string]struct{})
	}
	if _, exists := s.warningSeen[message]; exists {
		return
	}
	s.warningSeen[message] = struct{}{}
	s.warnings = append(s.warnings, message)
}

// TakeWarnings drains pending warnings.
func (s *Server) TakeWarnings() []string {
	s.warningsMu.Lock()
	defer s.warningsMu.Unlock()
	if len(s.warnings) == 0 {
		return nil
	}
	warnings := append([]string(nil), s.warnings...)
	s.warnings = nil
	return warnings
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
		daemonHost:         newDaemonHost(),
		listenPort:         defaultPort,
		sessionProjectPath: sessionProjectPath,
		warningSeen:        make(map[string]struct{}),
		annotationStore:    annotation.NewStore(10 * time.Minute),
		pushInbox:          push.NewPushInbox(50),
		ptyManager:         pty.NewManager(),
		tokenTracker:       tracking.NewTokenTracker(),
		intentStore:        terminal.NewIntentStore(),
		stateRecovery:      stateRecovery,
		incidents:          incident.NewStore(100, telemetry.QueueReliability),
	}

	// Create log store with warning callback wired to server
	s.logs = logstore.New(logstore.Config{
		LogFile:       logFile,
		MaxEntries:    maxEntries,
		TelemetryMode: telemetryModeAuto,
		AddWarning:    s.AddWarning,
		Stderrf:       diag.Printf,
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

	// Ensure log directory exists
	if s.logs.LogFile() != "" {
		dir := filepath.Dir(s.logs.LogFile())
		// #nosec G301 -- log directory: owner rwx, group rx for diagnostics
		if err := os.MkdirAll(dir, 0o750); err != nil {
			fallback := logstore.FallbackFilePath()
			s.AddWarning(fmt.Sprintf("state_dir_not_writable: %v; falling back to %s", err, fallback))
			s.logs.SetLogFile(fallback)
			_ = os.MkdirAll(filepath.Dir(s.logs.LogFile()), 0o750)
		}
		if err := logstore.EnsureFileWritable(s.logs.LogFile()); err != nil {
			fallback := logstore.FallbackFilePath()
			s.AddWarning(fmt.Sprintf("state_dir_not_writable: %v; falling back to %s", err, fallback))
			s.logs.SetLogFile(fallback)
			if err := os.MkdirAll(filepath.Dir(s.logs.LogFile()), 0o750); err != nil {
				s.AddWarning(fmt.Sprintf("log_persistence_disabled: %v", err))
				s.logs.SetLogFile("")
			} else if err := logstore.EnsureFileWritable(s.logs.LogFile()); err != nil {
				s.AddWarning(fmt.Sprintf("log_persistence_disabled: %v", err))
				s.logs.SetLogFile("")
			}
		}
	}

	// Load existing entries
	if s.logs.LogFile() != "" {
		if err := s.logs.LoadEntries(); err != nil {
			// File might not exist yet, that's OK
			if !os.IsNotExist(err) {
				s.AddWarning(fmt.Sprintf("log_load_failed: %v", err))
			}
		}
	}

	return s, nil
}

// setListenPort stores the active HTTP listener port for URL rewriting helpers.
func (s *Server) setListenPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if port > 0 {
		s.listenPort = port
	}
}

// getListenPort returns the active HTTP listener port.
func (s *Server) getListenPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listenPort <= 0 {
		return defaultPort
	}
	return s.listenPort
}

func (s *Server) getAnnotationStore() *annotation.Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.annotationStore == nil {
		s.annotationStore = annotation.NewStore(10 * time.Minute)
	}
	return s.annotationStore
}

func (s *Server) closeAnnotationStore() {
	if s == nil {
		return
	}
	store := func() *annotation.Store {
		s.mu.Lock()
		defer s.mu.Unlock()
		store := s.annotationStore
		s.annotationStore = nil
		return store
	}()
	if store != nil {
		store.Close()
	}
}

// terminalStatus is the diagnosable state of the terminal server.
//
// A terminal-port bind failure is non-fatal — the daemon serves MCP normally and
// the terminal is simply absent — so the ONLY way a user or agent learns what
// happened is this status. The daemon's stderr explanation cannot serve that role:
// a bridge-spawned daemon is started with Stdout/Stderr = nil (so it cannot die of
// SIGPIPE), which sends every stderr diagnostic to /dev/null.
type terminalStatus struct {
	Available        bool   `json:"available"`
	Port             int    `json:"port"`
	Error            string `json:"error,omitempty"`
	BlockedByPID     int    `json:"blocked_by_pid,omitempty"`
	BlockedByCommand string `json:"blocked_by_command,omitempty"`
}

// setTerminalPort stores the port the terminal server is listening on. A non-zero
// port means a successful bind, which clears any previous failure diagnosis — a
// stale "blocked by postgres" would be worse than reporting nothing. Port 0 is how
// the supervisor reports the server died, so availability follows it down.
func (s *Server) setTerminalPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terminalPort = port
	s.terminalAvailable = port > 0
	if port > 0 {
		s.terminalError = ""
		s.terminalBlockedByPID = 0
		s.terminalBlockedByCommand = ""
	}
}

// setTerminalUnavailable records WHY the terminal server is not running, including
// the process holding the port when we can identify it. "Port busy" is not
// actionable; "postgres (pid 4242) is on 7891" is.
func (s *Server) setTerminalUnavailable(port int, reason string, blockingPID int, blockingCommand string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terminalPort = 0 // nothing is listening for us
	s.terminalAvailable = false
	s.terminalError = reason
	s.terminalBlockedByPID = blockingPID
	s.terminalBlockedByCommand = blockingCommand
	s.terminalWantedPort = port
}

// getTerminalStatus returns the terminal server's diagnosable state.
func (s *Server) getTerminalStatus() terminalStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	port := s.terminalPort
	if !s.terminalAvailable && s.terminalWantedPort > 0 {
		// Report the port we could not get, so the message names something concrete.
		port = s.terminalWantedPort
	}
	return terminalStatus{
		Available:        s.terminalAvailable,
		Port:             port,
		Error:            s.terminalError,
		BlockedByPID:     s.terminalBlockedByPID,
		BlockedByCommand: s.terminalBlockedByCommand,
	}
}

// getTerminalPort returns the terminal server port (0 if not running).
func (s *Server) getTerminalPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terminalPort
}

// GetActiveCodebase returns the active codebase path (thread-safe).
func (s *Server) GetActiveCodebase() string {
	s.activeCodebaseMu.RLock()
	defer s.activeCodebaseMu.RUnlock()
	return s.activeCodebase
}

// SetActiveCodebase updates the active codebase path (thread-safe).
func (s *Server) SetActiveCodebase(path string) {
	s.activeCodebaseMu.Lock()
	defer s.activeCodebaseMu.Unlock()
	s.activeCodebase = path
}

// Close gracefully shuts down the server, draining the async log writer.
func (s *Server) Close() {
	s.logs.Shutdown(asyncLoggerDrainTimeout)
	s.closeAnnotationStore()
}

//go:embed openapi.json
var openapiJSON []byte

func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(openapiJSON); err != nil {
		diag.Printf("[kaboom] failed to write /openapi.json response: %v\n", err)
	}
}

func handleTelemetry(server *Server, captured *capture.Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		telemetryType := r.URL.Query().Get("type")
		if telemetryType == "" {
			httpapi.JSON(w, http.StatusBadRequest, map[string]string{
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
		switch telemetryType {
		case "logs":
			entries := server.logs.Entries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "network_waterfall":
			entries := captured.Telemetry().NetworkWaterfall().Entries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "network_bodies":
			entries := captured.Telemetry().NetworkBodies().Snapshot().Bodies
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "websocket_events":
			entries := captured.Telemetry().WebSockets().Events(types.WebSocketEventFilter{})
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "actions":
			entries := captured.Telemetry().Actions().Snapshot().Actions
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "performance_snapshots":
			entries := captured.Performance().Entries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "extension_logs":
			entries := captured.ExtensionLogs().Entries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result, count = entries, len(entries)
		case "websocket_status":
			status := captured.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
			httpapi.JSON(w, http.StatusOK, map[string]any{
				"type": telemetryType, "connections": status.Connections,
				"closed": status.Closed, "count": len(status.Connections),
			})
			return
		default:
			httpapi.JSON(w, http.StatusBadRequest, map[string]string{
				"error": "Unknown telemetry type: " + telemetryType,
				"hint":  "Valid types: logs, network_waterfall, network_bodies, websocket_events, actions, performance_snapshots, extension_logs, websocket_status",
			})
			return
		}
		httpapi.JSON(w, http.StatusOK, map[string]any{"type": telemetryType, "items": result, "count": count})
	}
}

func setupHTTPRoutes(server *Server, captured *capture.Capture) (*http.ServeMux, *MCPHandler) {
	server.mediaHTTP = mediaapi.New(captured, server.annotationStore, server.pushRouter)
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
	mux.HandleFunc("/sync", httpguard.CORS(httpguard.ExtensionOnly(capture.NewSyncHandler(captured).HandleSync)))
	registerClientRegistryRoutes(mux, captured)
	mux.HandleFunc("/recordings/save", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		screenrec.HandleSave(w, r, captured.Queries(), server.stateRecovery)
	})))
	mux.HandleFunc("/recordings/storage", httpguard.CORS(httpguard.ExtensionOnly(captureHTTP.HandleRecordingStorage)))
	mux.HandleFunc("/recordings/reveal", httpguard.CORS(httpguard.ExtensionOnly(screenrec.HandleReveal)))
	mux.HandleFunc("/telemetry", httpguard.CORS(handleTelemetry(server, captured)))
	mux.HandleFunc("/snapshot", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Snapshot(server.logs, captured))))
	mux.HandleFunc("/clear", httpguard.CORS(httpguard.ExtensionOnly(ciapi.Clear(server.logs, capture.NewStateResetter(captured)))))
	mux.HandleFunc("/test-boundary", httpguard.CORS(httpguard.ExtensionOnly(ciapi.TestBoundary(captured))))
}

func resolveClientRegistry(captured *capture.Capture, w http.ResponseWriter) (clientstore.Registry, bool) {
	registry := captured.Clients().Registry()
	if registry == nil {
		httpapi.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "client_registry_unavailable"})
		return nil, false
	}
	return registry, true
}

func registerClientRegistryRoutes(mux *http.ServeMux, captured *capture.Capture) {
	mux.HandleFunc("/clients", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientsList(w, r, captured)
	})))
	mux.HandleFunc("/clients/", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientByID(w, r, captured)
	})))
}

func handleClientsList(w http.ResponseWriter, r *http.Request, captured *capture.Capture) {
	registry, ok := resolveClientRegistry(captured, w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		httpapi.JSON(w, http.StatusOK, map[string]any{"clients": registry.List(), "count": registry.Count()})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
		var body struct {
			CWD string `json:"cwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		httpapi.JSON(w, http.StatusOK, map[string]any{"result": registry.Register(body.CWD)})
	default:
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func handleClientByID(w http.ResponseWriter, r *http.Request, captured *capture.Capture) {
	registry, ok := resolveClientRegistry(captured, w)
	if !ok {
		return
	}
	clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
	if clientID == "" {
		httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		client := registry.Get(clientID)
		if client == nil {
			httpapi.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		httpapi.JSON(w, http.StatusOK, client)
	case http.MethodDelete:
		if !registry.Unregister(clientID) {
			httpapi.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		httpapi.JSON(w, http.StatusOK, map[string]bool{"unregistered": true})
	default:
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
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
	mux.HandleFunc("/openapi.json", httpguard.CORS(handleOpenAPI))

	mcpHandler := NewToolHandler(server, captured)
	mux.HandleFunc("/mcp", httpguard.CORS(newMCPHTTPHandler(mcpHandler).ServeHTTP))
	operations := operationalapi.New(operationalapi.Options{
		Logs:      server.logs,
		Capture:   captured,
		Version:   version,
		StartedAt: server.runtime.StartedAt(),
		TerminalStatus: func() operationalapi.TerminalStatus {
			status := server.getTerminalStatus()
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
		UsageTracker:    mcpHandler.GetUsageTracker,
		MaxPostBodySize: maxPostBodySize,
	})

	mux.HandleFunc("/api/status", httpguard.CORS(dashboard.Status(dashboard.StatusOptions{
		Version: version, StartedAt: server.runtime.StartedAt(), Capture: captured, JSONResponse: httpapi.JSON,
		Logs: func() (int, int) {
			return server.logs.EntryCount(), server.logs.MaxEntries()
		},
		Terminal: func() (int, int, []string) {
			port := server.getTerminalPort()
			if server.ptyManager == nil {
				return port, 0, nil
			}
			return port, server.ptyManager.Count(), server.ptyManager.List()
		},
		ListenPort: server.getListenPort,
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
			extraChecks = recoveryDoctorChecks(handler.stateRecovery)
			extraChecks = append(extraChecks, incidentDoctorChecks(server.incidents)...)
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
