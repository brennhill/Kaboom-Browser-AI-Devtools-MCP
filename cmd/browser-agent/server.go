// Purpose: Defines the Server struct and startup wiring for log, push, and annotation subsystems.
// Why: Centralizes top-level server state while detailed persistence and logging mechanics live in focused modules.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tracking"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Server holds the server state.
type Server struct {
	listenPort int
	mu         sync.RWMutex

	// Log subsystem — owns entries, TTL rotation, async channel, file persistence.
	logs *logstore.Store

	// One-shot warnings surfaced via MCP tool responses.
	warningsMu  sync.Mutex
	warnings    []string
	warningSeen map[string]struct{}

	// Annotation store is server-scoped to avoid cross-session contamination.
	annotationStore *AnnotationStore

	// Push delivery pipeline
	pushInbox  *push.PushInbox
	pushRouter *push.Router

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
	terminalSupervisor *terminalSupervisor

	// Active codebase path — set via MCP configure(what='store', key='active_codebase')
	// or via the extension options page. Used as default CWD for terminal sessions.
	activeCodebaseMu sync.RWMutex
	activeCodebase   string

	// Token savings tracker for output compression hooks.
	tokenTracker *tracking.TokenTracker

	// Push drain authentication token. When non-empty, /push/drain requires
	// Authorization: Bearer <token>. Set via --push-drain-token flag.
	pushDrainToken string

	// Screenshot rate limiting: prevent DoS by limiting uploads to 1/second per client
	screenshotRateLimiter map[string]time.Time
	screenshotRateMu      sync.Mutex
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

// NewServer creates a new server instance.
func NewServer(logFile string, maxEntries int) (*Server, error) {
	s := &Server{
		listenPort:            defaultPort,
		warningSeen:           make(map[string]struct{}),
		annotationStore:       NewAnnotationStore(10 * time.Minute),
		pushInbox:             push.NewPushInbox(50),
		ptyManager:            pty.NewManager(),
		tokenTracker:          tracking.NewTokenTracker(),
		intentStore:           terminal.NewIntentStore(),
		screenshotRateLimiter: make(map[string]time.Time),
	}

	// Create log store with warning callback wired to server
	s.logs = logstore.New(logstore.Config{
		LogFile:       logFile,
		MaxEntries:    maxEntries,
		TelemetryMode: telemetryModeAuto,
		AddWarning:    s.AddWarning,
		Stderrf:       stderrf,
	})

	// Initialize push router with capability sync callback
	caps := getPushClientCapabilities()
	s.pushRouter = push.NewRouter(s.pushInbox, &stdioSamplingSender{}, &stdioNotifier{}, caps)
	onPushCapabilitiesChange(func(newCaps push.ClientCapabilities) {
		s.pushRouter.UpdateCapabilities(newCaps)
	})

	// Start async logger goroutine
	util.SafeGo(func() { s.logs.RunWorker() })

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

func (s *Server) getAnnotationStore() *AnnotationStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.annotationStore == nil {
		s.annotationStore = NewAnnotationStore(10 * time.Minute)
	}
	return s.annotationStore
}

func (s *Server) closeAnnotationStore() {
	if s == nil {
		return
	}
	store := func() *AnnotationStore {
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
