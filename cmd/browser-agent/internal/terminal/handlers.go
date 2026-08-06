// handlers.go -- HTTP handlers for the in-browser terminal.
// Why: Isolates terminal WebSocket upgrade, session lifecycle, and static asset serving
// from the main route wiring for maintainability and test focus.
// Docs: docs/features/feature/terminal/index.md

package terminal

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	terminalassets "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/assets"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/dimensions"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/directorybrowser"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/spawnpolicy"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/wstransport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	ptydiag "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty/diagnostics"
	ptyupload "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// defaultShell returns a usable interactive shell for the host. It prefers
// $SHELL, then falls back to shells that actually exist. Hardcoding a single
// path (e.g. /bin/zsh) fails on hosts without it, such as Linux CI runners and
// minimal containers.
func defaultShell() string {
	if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
		if _, err := exec.LookPath(sh); err == nil {
			return sh
		}
	}
	for _, candidate := range []string{"/bin/zsh", "/bin/bash", "/bin/sh"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return "/bin/sh"
}

// resolveTerminalCommand starts the implicit interactive shell as a login
// shell so daemons launched with a minimal service environment load the user's
// profile (including PATH). Explicit commands retain their exact argument list.
func resolveTerminalCommand(cmd string, args []string) (string, []string) {
	if cmd != "" {
		return cmd, args
	}
	return defaultShell(), []string{"-l"}
}

// IdleTimeout is the duration of silence after PTY output before
// the idle callback fires. Used to detect when an agent is waiting for input.
const IdleTimeout = 30 * time.Second

// RegisterRoutes adds terminal-related routes to the mux.
// NOT MCP — These are daemon-served endpoints for the in-browser terminal.
func RegisterRoutes(mux *http.ServeMux, deps Deps, server ServerDeps, mgr *pty.Manager, cap *capture.Capture) *sessionrelay.Map {
	relays := sessionrelay.NewMap()

	// Route a stuck-writer write-buffer close timeout (a drain goroutine + fd leak
	// that cannot be safely interrupted) to the structured log, so the leak is
	// diagnosable instead of silent (finding M).
	// Route PTY-internal state-mutating failures (a child that will not die, a PTY
	// fd that will not close, a stranded stdin flush) to the structured log. The
	// pty package has no logger of its own, so without this sink those failures are
	// invisible in production (finding S8, rule 25).
	ptydiag.SetHook(func(event string, fields map[string]any) {
		deps.logEvent(event, fields)
	})

	pty.SetWriteBufferCloseTimeoutHook(func(pending int) {
		deps.logEvent("terminal_writebuffer_close_timeout", map[string]any{"pending_bytes": pending})
		if deps.Stderrf != nil {
			deps.Stderrf("[Kaboom] terminal write buffer close timed out (%d bytes undrained; drain goroutine+fd leaked)\n", pending)
		}
	})

	// Serve terminal HTML page.
	mux.HandleFunc("/terminal", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalPage(w, r, deps)
	}))

	// Serve xterm.js and other static assets.
	staticFS, err := fs.Sub(terminalassets.FS, "terminal_assets")
	if err != nil {
		deps.Stderrf("[Kaboom] failed to create terminal static FS: %v\n", err)
		return relays
	}
	mux.Handle("/terminal/static/", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/terminal/static")
		http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
	}))

	// WebSocket upgrade for PTY I/O.
	mux.HandleFunc("/terminal/ws", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		wstransport.Handle(w, r, wstransport.Deps{
			JSONResponse: deps.JSONResponse,
			Stderrf:      deps.Stderrf, LogEvent: deps.LogEvent,
			WSReadFrame: deps.WSReadFrame, WSWriteFrame: deps.WSWriteFrame, WSAcceptKey: deps.WSAcceptKey,
		}, mgr, relays)
	}))

	// Session lifecycle.
	mux.HandleFunc("/terminal/start", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalStart(w, r, deps, server, mgr, cap, relays)
	}))
	mux.HandleFunc("/terminal/stop", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalStop(w, r, deps, mgr, relays)
	}))

	// Session validation — checks a specific token against a live session.
	mux.HandleFunc("/terminal/validate", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalValidate(w, r, deps, mgr)
	}))

	// Session configuration.
	mux.HandleFunc("/terminal/config", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalConfig(w, r, deps, mgr, relays)
	}))

	// Directory listing for the side panel's root-folder picker. The browser
	// cannot resolve an absolute path on its own, so the daemon does it.
	mux.HandleFunc("/terminal/dirs", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		directorybrowser.Handle(w, r, deps.JSONResponse)
	}))

	// Image upload for terminal sessions.
	mux.HandleFunc("/terminal/upload", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalUpload(w, r, deps, mgr, relays)
	}))

	// NOTE: /config/active-codebase is registered in registerCoreRoutes (not terminal-specific).
	return relays
}

// HandleTerminalPage serves the terminal HTML page.
func HandleTerminalPage(w http.ResponseWriter, r *http.Request, deps Deps) {
	if r.Method != "GET" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	data, err := terminalassets.FS.ReadFile("terminal_assets/terminal.html")
	if err != nil {
		deps.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to read terminal page"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HandleTerminalStart creates a new terminal session.
// resolveStartDir applies the terminal CWD priority: an explicit request dir
// wins, else the active codebase (set via MCP/extension), else auto-detection.
// autoDetect is a thunk so the (capture-dependent) fallback is only computed when
// the higher-priority sources are empty. Pure and unit-testable in isolation.
func resolveStartDir(reqDir, activeCodebase string, autoDetect func() string) string {
	if reqDir != "" {
		return reqDir
	}
	if activeCodebase != "" {
		return activeCodebase
	}
	return autoDetect()
}

// classifyStartError maps a mgr.Start failure to an HTTP status + JSON body.
// Getting this right is load-bearing: only "session already exists" is a benign
// reconnect (409 + the live token so the client attaches instead of killing it);
// every other failure — bad cwd, spawn error, session limit — must surface with a
// distinct status. Bucketing them all as 409-with-token once silently reconnected
// the client to the *old* cwd with no error ("terminal failed for no reason").
func classifyStartError(err error, sessionID, token string) (int, map[string]any) {
	// fork/exec EPERM: the OS refused to spawn a child. A restricted sandbox profile
	// is the usual cause, but it is an INFERENCE — sandboxPayload states it as such
	// and always carries the underlying error in `detail` (see IsSandboxError).
	if spawnpolicy.IsSandboxError(err) {
		return http.StatusServiceUnavailable, sandboxPayload(err)
	}
	if errors.Is(err, pty.ErrSessionExists) {
		return http.StatusConflict, map[string]any{
			"error":      err.Error(),
			"session_id": sessionID,
			"token":      token,
		}
	}
	status := http.StatusBadRequest
	if errors.Is(err, pty.ErrMaxSessions) {
		status = http.StatusTooManyRequests
	}
	return status, map[string]any{
		"error":      err.Error(),
		"session_id": sessionID,
	}
}

func HandleTerminalStart(w http.ResponseWriter, r *http.Request, deps Deps, server ServerDeps, mgr *pty.Manager, cap *capture.Capture, relays *sessionrelay.Map) {
	if r.Method != "POST" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, deps.MaxPostBody)

	var req struct {
		ID          string   `json:"id"`
		Cmd         string   `json:"cmd"`
		Args        []string `json:"args"`
		Dir         string   `json:"dir"`
		Cols        int      `json:"cols"`
		Rows        int      `json:"rows"`
		InitCommand string   `json:"init_command"`
		RepoPath    string   `json:"repo_path"`
		AgentType   string   `json:"agent_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	cols, rows, validDimensions := dimensions.Resolve(req.Cols, req.Rows)
	if !validDimensions {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "cols and rows must be between 0 and 65535"})
		return
	}

	req.Cmd, req.Args = resolveTerminalCommand(req.Cmd, req.Args)

	// CWD priority: request dir > active_codebase (set via MCP/extension) > auto-detect
	activeCodebase := ""
	if server != nil {
		activeCodebase = server.GetActiveCodebase()
	}
	req.Dir = resolveStartDir(req.Dir, activeCodebase, func() string {
		if cap == nil {
			return ""
		}
		return AutoDetectCWD(cap)
	})

	startCfg := pty.StartConfig{
		ID:        req.ID,
		Cmd:       req.Cmd,
		Args:      req.Args,
		Dir:       req.Dir,
		Cols:      cols,
		Rows:      rows,
		RepoPath:  req.RepoPath,
		AgentType: req.AgentType,
	}
	// A fork/exec EPERM is often transient (fork pressure, AV/EDR interposition), so
	// retry it briefly rather than sending the user to a "restart your daemon" dead
	// end. Only that one typed error is retried; everything else fails immediately.
	// The sleep hook doubles as the retry log — otherwise a recovered spawn would
	// leave no trace and this whole path would be invisible in production.
	result, err := spawnpolicy.StartWithEPERMRetry(
		func() (*pty.StartResult, error) { return mgr.Start(startCfg) },
		func(d time.Duration) {
			deps.logEvent("terminal_spawn_eperm_retry", map[string]any{
				"session_id": req.ID,
				"dir":        req.Dir,
				"backoff_ms": d.Milliseconds(),
			})
			time.Sleep(d)
		},
	)
	// On success: create relay (fan-out + write buffer), configure idle detection,
	// and handle init_command via the relay instead of reading PTY directly.
	if err == nil {
		// Use the session Start returned rather than looking it up again by ID: the
		// old `sess, _ := mgr.Get(result.SessionID)` swallowed the error and then
		// dereferenced the result, so a /terminal/stop landing between Start and Get
		// nil-panicked this handler (finding S4).
		sess := result.Session
		var relay *sessionrelay.Relay
		if result.Replaced {
			// The manager evicted a dead session with this ID and spawned a fresh
			// one. Atomically drop the stale relay (closed fanout, bound to the dead
			// session) and install a new relay + readLoop bound to the new session.
			// ReplaceRelay does this under one lock so a concurrent WS GetOrCreate
			// cannot bind the relay to the dead session in the gap (finding H) —
			// otherwise the reconnecting browser would attach to a dead fanout.
			relay = relays.ReplaceRelay(result.SessionID, sess, req.Dir)
			deps.logEvent("terminal_session_healed", map[string]any{
				"session_id": result.SessionID,
				"pid":        result.Pid,
			})
		} else {
			relay = relays.GetOrCreate(result.SessionID, sess, req.Dir)
		}
		deps.logEvent("terminal_session_spawned", map[string]any{
			"session_id": result.SessionID,
			"pid":        result.Pid,
			"dir":        req.Dir,
		})
		sess.SetIdleConfig(pty.IdleConfig{
			Timeout: IdleTimeout,
			Callback: func(id string) {
				deps.Stderrf("[Kaboom] terminal session %s is idle\n", id)
				deps.logEvent("terminal_session_idle", map[string]any{"session_id": id})
			},
		})
		if req.InitCommand != "" {
			// Panic-recovered one-shot init (bounded by InitTimeout): a panic here
			// must never crash the daemon.
			r, cmd := relay, req.InitCommand
			util.SafeGo(func() { sessionrelay.WaitForPromptViaRelay(r, cmd) })
		}
	}
	if err != nil {
		sessionID := req.ID
		if sessionID == "" {
			sessionID = "default"
		}
		status, body := classifyStartError(err, sessionID, mgr.GetTokenForSession(sessionID))
		// Spawning a session is state-mutating: a failure must be logged (rule 25,
		// fail-loud) so a "terminal won't start" report is diagnosable from
		// ~/.kaboom/logs/kaboom.jsonl, not just surfaced transiently to the client.
		// A 409 (session already exists) is a benign reconnect, not a failure.
		if status != http.StatusConflict {
			deps.logEvent("terminal_session_start_failed", map[string]any{
				"session_id": sessionID,
				"dir":        req.Dir,
				"status":     status,
				"error":      err.Error(),
			})
		}
		deps.JSONResponse(w, status, body)
		return
	}

	deps.JSONResponse(w, http.StatusOK, map[string]any{
		"session_id": result.SessionID,
		"token":      result.Token,
		"pid":        result.Pid,
	})
}

// AutoDetectCWD gets the CWD from the first registered MCP client.
func AutoDetectCWD(cap *capture.Capture) string {
	reg := cap.Clients().Registry()
	if reg == nil {
		return ""
	}
	clients := reg.List()
	if clients == nil {
		return ""
	}

	// List() returns any — extract CWD from the first client.
	switch v := clients.(type) {
	case []any:
		for _, c := range v {
			if m, ok := c.(map[string]any); ok {
				if cwd, ok := m["cwd"].(string); ok && cwd != "" {
					return cwd
				}
			}
		}
	default:
		// Try JSON roundtrip as fallback.
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		var entries []map[string]any
		if err := json.Unmarshal(data, &entries); err != nil {
			return ""
		}
		for _, e := range entries {
			if cwd, ok := e["cwd"].(string); ok && cwd != "" {
				return cwd
			}
		}
	}
	return ""
}

// HandleTerminalStop destroys a terminal session.
func HandleTerminalStop(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *sessionrelay.Map) {
	if r.Method != "POST" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, deps.MaxPostBody)

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.ID == "" {
		req.ID = "default"
	}

	if err := mgr.Stop(req.ID); err != nil {
		// Stopping a session is state-mutating; a failure (e.g. session gone) is
		// logged so an unexpected teardown outcome is diagnosable, not silent.
		deps.logEvent("terminal_session_stop_failed", map[string]any{
			"session_id": req.ID,
			"error":      err.Error(),
		})
		deps.JSONResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	relays.Remove(req.ID)

	deps.JSONResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// sandboxPayload builds the 503 body for a fork/exec EPERM. The diagnosis is an
// inference; the underlying error is the fact, so `detail` always carries it.
func sandboxPayload(err error) map[string]any {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return map[string]any{
		"error":       "sandbox_restricted",
		"message":     "The daemon could not spawn a terminal process: the OS denied fork/exec. This usually means the daemon is running under a restricted sandbox profile.",
		"instruction": "Run this command in a separate terminal to restart the daemon with full permissions:",
		"command":     "kaboom-agentic-browser --stop && kaboom-agentic-browser --daemon",
		"detail":      detail,
	}
}

// HandleTerminalConfig returns terminal session details including alt-screen state and subscriber counts.
func HandleTerminalConfig(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *sessionrelay.Map) {
	switch r.Method {
	case "GET":
		ids := mgr.List()
		sessions := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			sess, err := mgr.Get(id)
			if err != nil {
				continue
			}
			info := map[string]any{
				"id":         id,
				"alive":      sess.IsAlive(),
				"pid":        sess.Pid(),
				"alt_screen": sess.AltScreenActive(),
			}
			if relay := relays.Get(id); relay != nil {
				info["subscribers"] = relay.Fanout().Count()
			}
			sessions = append(sessions, info)
		}
		deps.JSONResponse(w, http.StatusOK, map[string]any{
			"sessions": sessions,
			"count":    mgr.Count(),
		})
	default:
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}

// HandleTerminalValidate checks whether a specific token maps to a live PTY session.
// Returns {"valid": true} if the token resolves to a running session, false otherwise.
func HandleTerminalValidate(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager) {
	if r.Method != "GET" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		deps.JSONResponse(w, http.StatusOK, map[string]bool{"valid": false})
		return
	}
	sess, err := mgr.GetByToken(token)
	if err != nil {
		deps.JSONResponse(w, http.StatusOK, map[string]bool{"valid": false})
		return
	}
	deps.JSONResponse(w, http.StatusOK, map[string]bool{"valid": sess.IsAlive()})
}

// HandleTerminalUpload handles image uploads for terminal sessions.
// POST /terminal/upload?session_id=xxx&filename=screenshot.png
// Content-Type must be an image type. Body is raw image data.
func HandleTerminalUpload(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *sessionrelay.Map) {
	if r.Method != "POST" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = "default"
	}

	// Cap request body at the upload limit (+4KB for overhead) to prevent
	// unbounded memory buffering before ptyupload.Upload's own LimitReader kicks in.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20+4096)

	// Verify session exists.
	if _, err := mgr.Get(sessionID); err != nil {
		deps.JSONResponse(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	relay := relays.Get(sessionID)
	workspaceDir := ""
	if relay != nil {
		workspaceDir = relay.WorkspaceDir()
	}
	if workspaceDir == "" {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "no workspace directory for session"})
		return
	}

	contentType := r.Header.Get("Content-Type")
	filename := r.URL.Query().Get("filename")

	result, err := ptyupload.Upload(workspaceDir, sessionID, contentType, filename, r.Body)
	if err != nil {
		status := http.StatusBadRequest
		if err == ptyupload.ErrUploadTooLarge {
			status = http.StatusRequestEntityTooLarge
		}
		deps.JSONResponse(w, status, map[string]string{"error": err.Error()})
		return
	}

	deps.JSONResponse(w, http.StatusOK, map[string]any{
		"path": result.RelPath,
		"size": result.Size,
	})
}

// HandleActiveCodebase gets or sets the active codebase path used as terminal CWD.
func HandleActiveCodebase(w http.ResponseWriter, r *http.Request, deps Deps, server ServerDeps) {
	switch r.Method {
	case "GET":
		deps.JSONResponse(w, http.StatusOK, map[string]string{
			"active_codebase": server.GetActiveCodebase(),
		})
	case "PUT", "POST":
		r.Body = http.MaxBytesReader(w, r.Body, deps.MaxPostBody)
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		server.SetActiveCodebase(strings.TrimSpace(body.Path))
		deps.JSONResponse(w, http.StatusOK, map[string]string{
			"status":          "ok",
			"active_codebase": server.GetActiveCodebase(),
		})
	default:
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}
