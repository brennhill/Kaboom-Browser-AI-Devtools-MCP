// handlers.go -- HTTP handlers for the in-browser terminal.
// Why: Isolates terminal WebSocket upgrade, session lifecycle, and static asset serving
// from the main route wiring for maintainability and test focus.
// Docs: docs/features/feature/terminal/index.md

package terminal

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
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

// PingInterval is how often the server sends WebSocket ping frames.
// Browser WebSocket API auto-replies with pong — no client code needed.
const PingInterval = 30 * time.Second

// PongTimeout is the max time allowed without receiving any frame (data or pong).
// If exceeded, the connection is considered dead and closed. The PTY session survives
// so the browser can reconnect with scrollback replay.
const PongTimeout = 60 * time.Second

// ReadBufSize is the buffer size for PTY reads relayed to the browser.
const ReadBufSize = 4096

// InitTimeout is the max time to wait for a shell prompt before
// writing init_command. Replaces the old hardcoded 500ms sleep with an
// adaptive readiness check that looks for prompt characters.
const InitTimeout = 2 * time.Second

// PromptChars contains characters that indicate a shell prompt is ready.
const PromptChars = "$#>%"

// IdleTimeout is the duration of silence after PTY output before
// the idle callback fires. Used to detect when an agent is waiting for input.
const IdleTimeout = 30 * time.Second

// RegisterRoutes adds terminal-related routes to the mux.
// NOT MCP — These are daemon-served endpoints for the in-browser terminal.
func RegisterRoutes(mux *http.ServeMux, deps Deps, server ServerDeps, mgr *pty.Manager, cap *capture.Store) *Map {
	relays := NewMap()

	// Serve terminal HTML page.
	mux.HandleFunc("/terminal", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalPage(w, r, deps)
	}))

	// Serve xterm.js and other static assets.
	staticFS, err := fs.Sub(AssetsFS, "terminal_assets")
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
		HandleTerminalWS(w, r, deps, mgr, relays)
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
	data, err := AssetsFS.ReadFile("terminal_assets/terminal.html")
	if err != nil {
		deps.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to read terminal page"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HandleTerminalWS upgrades a GET /terminal/ws request to a WebSocket connection
// that relays raw PTY I/O to/from the browser's xterm.js terminal emulator.
func HandleTerminalWS(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *Map) {
	token := r.URL.Query().Get("token")
	if token == "" {
		deps.JSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}

	sess, err := mgr.GetByToken(token)
	if err != nil {
		deps.JSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" || strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "websocket upgrade required"})
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		deps.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "server does not support hijacking"})
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		deps.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// NOTE: After a successful handshake, conn.Close() is handled by closeConn
	// inside wsLoop via sync.Once. We only close here on handshake failure.

	// Send 101 handshake.
	accept := deps.WSAcceptKey(key)
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := bufrw.WriteString(handshake); err != nil {
		_ = conn.Close()
		return
	}
	if err := bufrw.Flush(); err != nil {
		_ = conn.Close()
		return
	}

	// Get or create relay for multi-subscriber fan-out.
	relay := relays.GetOrCreate(sess.ID, sess, "")

	// Capture scrollback BEFORE subscribing to the fanout to avoid duplicate
	// data. The readLoop appends to scrollback then broadcasts, so any data
	// arriving after this snapshot will be delivered only via the subscriber
	// channel, not replayed from scrollback.
	history := sess.Scrollback()

	subID := NextWSSubID()
	sub, subErr := relay.fanout.Subscribe(subID)

	// Replay scrollback so the reconnecting (or first-connecting) terminal sees prior output.
	if len(history) > 0 {
		for off := 0; off < len(history); off += ReadBufSize {
			end := off + ReadBufSize
			if end > len(history) {
				end = len(history)
			}
			if err := deps.WSWriteFrame(bufrw, 0x2, history[off:end]); err != nil {
				if subErr == nil {
					relay.fanout.Unsubscribe(subID)
				}
				_ = conn.Close()
				return
			}
		}
	}
	replayEnd, _ := json.Marshal(map[string]string{"type": "replay_end"})
	if err := deps.WSWriteFrame(bufrw, 0x1, replayEnd); err != nil {
		if subErr == nil {
			relay.fanout.Unsubscribe(subID)
		}
		_ = conn.Close()
		return
	}

	// If subscribe failed (fanout already closed — process exited before reconnect),
	// send the final scrollback, exit notification, and close. The browser receives
	// the full output history plus the exited message and stops reconnecting.
	if subErr != nil {
		exitMsg, _ := json.Marshal(map[string]any{"type": "exited", "code": relay.exitCode})
		_ = deps.WSWriteFrame(bufrw, 0x1, exitMsg)
		_ = deps.WSWriteFrame(bufrw, 0x8, nil)
		_ = conn.Close()
		return
	}

	wsLoop(conn, bufrw, deps, sess, relay, sub)
	relay.fanout.Unsubscribe(subID)
}

// wsLoop relays data between a WebSocket connection and a PTY session.
// Downstream (fan-out -> browser): binary WebSocket frames with raw terminal output.
// Upstream (browser -> PTY): binary frames as keystrokes via write buffer, text frames as JSON control.
// Server sends ping frames every PingInterval; if no frame (data or pong) arrives
// within PongTimeout the connection is considered dead and closed. The PTY session
// survives so the browser can reconnect with scrollback replay.
//
// Coordinated shutdown: all three goroutines share a connDone channel and a sync.Once-guarded
// closeConn function. Any goroutine that detects a terminal condition calls closeConn(),
// which closes connDone (unblocking the others) then closes the underlying TCP connection
// exactly once. This prevents double-close races and goroutine leaks.
func wsLoop(conn net.Conn, rw *bufio.ReadWriter, deps Deps, sess *pty.Session, relay *Relay, sub <-chan []byte) {
	// Coordinated shutdown: connDone signals all goroutines to exit,
	// closeConn ensures conn.Close() is called exactly once.
	connDone := make(chan struct{})
	var connDoneOnce sync.Once
	closeConn := func() {
		connDoneOnce.Do(func() {
			close(connDone)
			_ = conn.Close()
		})
	}

	// Multiple goroutines emit frames (downstream, keepalive ping, and upstream
	// control responses). Serialize writes to avoid interleaved/corrupted frames.
	writeFrame := NewFrameWriter(rw, deps)

	// Fan-out -> WebSocket (downstream): read from subscriber channel and send as binary frames.
	// Also tracks alt-screen state changes and notifies the frontend.
	downstreamDone := make(chan struct{})
	go func() { // lint:allow-bare-goroutine — bounded by connDone/channel close
		defer close(downstreamDone)
		prevAltScreen := sess.AltScreenActive()
		for {
			select {
			case data, ok := <-sub:
				if !ok {
					// Fanout closed (session ended) — send exit notification
					// so the browser can display the message and stop reconnecting.
					exitMsg, _ := json.Marshal(map[string]any{"type": "exited", "code": relay.exitCode})
					_ = writeFrame(0x1, exitMsg)
					_ = writeFrame(0x8, nil)
					closeConn()
					return
				}
				if err := writeFrame(0x2, data); err != nil {
					closeConn()
					return
				}
				// Notify frontend of alt-screen state changes.
				altScreen := sess.AltScreenActive()
				if altScreen != prevAltScreen {
					prevAltScreen = altScreen
					ctrl, _ := json.Marshal(map[string]any{"type": "alt_screen", "active": altScreen})
					_ = writeFrame(0x1, ctrl)
				}
			case <-connDone:
				return
			}
		}
	}()

	// Server-initiated ping keepalive — detects dead connections (browser crash,
	// laptop sleep) without ever timing out idle users.
	pingTicker := time.NewTicker(PingInterval)
	go func() { // lint:allow-bare-goroutine — bounded by connDone
		defer pingTicker.Stop()
		for {
			select {
			case <-connDone:
				return
			case <-pingTicker.C:
				if err := writeFrame(0x9, nil); err != nil {
					closeConn()
					return
				}
			}
		}
	}()

	// WebSocket -> PTY (upstream): read frames and dispatch.
	// Uses relay.writeBuf for non-blocking writes with backpressure.
	// NOTE: Do NOT call sess.Close() on WebSocket disconnect — the session
	// must survive page refreshes so the browser can reconnect with scrollback replay.
	// Sessions are only killed explicitly via POST /terminal/stop (the Exit button).
	go func() { // lint:allow-bare-goroutine — bounded by connDone
		defer closeConn() // Close conn on exit so downstream detects it and browser auto-reconnects
		for {
			// Refresh read deadline on every iteration — any received frame
			// (data, pong, ping) proves the connection is alive.
			_ = conn.SetReadDeadline(time.Now().Add(PongTimeout))

			fin, opcode, payload, err := deps.WSReadFrame(rw)
			if err != nil {
				// Read deadline expired or connection error — close silently.
				// PTY stays alive for reconnection.
				return
			}

			// Reject fragmented frames (FIN=0). Terminal messages are always
			// single-frame; accepting fragments would require reassembly state
			// and risks incomplete data being written to the PTY.
			if !fin {
				_ = writeFrame(0x8, nil) // Send close frame per RFC 6455.
				return
			}

			switch opcode {
			case 0x8: // Close
				_ = writeFrame(0x8, nil)
				return // WebSocket closed — stop relaying but keep PTY alive
			case 0x9: // Ping -> Pong
				_ = writeFrame(0xA, payload)
			case 0xA: // Pong — no-op, deadline already refreshed above
			case 0x2: // Binary — raw keystrokes -> PTY stdin via write buffer
				_, _ = relay.writeBuf.Write(payload)
			case 0x1: // Text — JSON control message
				HandleControlMessage(payload, sess)
			}
		}
	}()

	<-downstreamDone
}

// NewFrameWriter returns a thread-safe frame writer for one WebSocket
// connection. All callers for that connection must share this writer.
func NewFrameWriter(rw *bufio.ReadWriter, deps Deps) func(opcode byte, payload []byte) error {
	var wsWriteMu sync.Mutex
	return func(opcode byte, payload []byte) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		return deps.WSWriteFrame(rw, opcode, payload)
	}
}

// ControlMessage is a JSON control message from the browser terminal.
type ControlMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// HandleControlMessage processes a JSON control message from the browser.
func HandleControlMessage(payload []byte, sess *pty.Session) {
	var msg ControlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "resize":
		if msg.Cols > 0 && msg.Rows > 0 {
			_ = sess.Resize(uint16(msg.Cols), uint16(msg.Rows))
			// Always force SIGWINCH so TUI apps redraw — TIOCSWINSZ only
			// sends SIGWINCH when dimensions actually change, but on reconnect
			// the dimensions may match while the display is stale.
			sess.ForceRedraw()
		}
	}
}

// HandleTerminalStart creates a new terminal session.
func HandleTerminalStart(w http.ResponseWriter, r *http.Request, deps Deps, server ServerDeps, mgr *pty.Manager, cap *capture.Store, relays *Map) {
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

	// Default to shell if no command specified.
	if req.Cmd == "" {
		req.Cmd = defaultShell()
	}

	// CWD priority: request dir > active_codebase (set via MCP/extension) > auto-detect
	if req.Dir == "" && server != nil {
		req.Dir = server.GetActiveCodebase()
	}
	if req.Dir == "" && cap != nil {
		req.Dir = AutoDetectCWD(cap)
	}

	result, err := mgr.Start(pty.StartConfig{
		ID:        req.ID,
		Cmd:       req.Cmd,
		Args:      req.Args,
		Dir:       req.Dir,
		Cols:      uint16(req.Cols),
		Rows:      uint16(req.Rows),
		RepoPath:  req.RepoPath,
		AgentType: req.AgentType,
	})
	// On success: create relay (fan-out + write buffer), configure idle detection,
	// and handle init_command via the relay instead of reading PTY directly.
	if err == nil {
		sess, _ := mgr.Get(result.SessionID)
		relay := relays.GetOrCreate(result.SessionID, sess, req.Dir)
		sess.SetIdleConfig(pty.IdleConfig{
			Timeout: IdleTimeout,
			Callback: func(id string) {
				deps.Stderrf("[Kaboom] terminal session %s is idle\n", id)
			},
		})
		if req.InitCommand != "" {
			go func(r *Relay, cmd string) { // lint:allow-bare-goroutine — one-shot init, bounded by timeout
				WaitForPromptViaRelay(r, cmd)
			}(relay, req.InitCommand)
		}
	}
	if err != nil {
		// Detect macOS sandbox restriction (MCP stdio-spawned daemon can't fork).
		if IsSandboxError(err) {
			deps.JSONResponse(w, http.StatusServiceUnavailable, map[string]any{
				"error":       "sandbox_restricted",
				"message":     "The daemon was started by an MCP client and cannot spawn terminal processes due to macOS sandbox restrictions.",
				"instruction": "Run this command in a separate terminal to restart the daemon with full permissions:",
				"command":     "kaboom-agentic-browser --stop && kaboom-agentic-browser --daemon",
			})
			return
		}
		// Return existing session's token so the client can reconnect instead of killing it.
		sessionID := req.ID
		if sessionID == "" {
			sessionID = "default"
		}
		existingToken := mgr.GetTokenForSession(sessionID)
		deps.JSONResponse(w, http.StatusConflict, map[string]any{
			"error":      err.Error(),
			"session_id": sessionID,
			"token":      existingToken,
		})
		return
	}

	deps.JSONResponse(w, http.StatusOK, map[string]any{
		"session_id": result.SessionID,
		"token":      result.Token,
		"pid":        result.Pid,
	})
}

// AutoDetectCWD gets the CWD from the first registered MCP client.
func AutoDetectCWD(cap *capture.Store) string {
	reg := cap.GetClientRegistry()
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
func HandleTerminalStop(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *Map) {
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
		deps.JSONResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	relays.Remove(req.ID)

	deps.JSONResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// IsSandboxError returns true if err looks like a macOS sandbox/fork restriction.
// MCP stdio-spawned daemons inherit a restricted environment that blocks posix_spawn/fork.
func IsSandboxError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "Operation not permitted") ||
		strings.Contains(msg, "not permitted")
}

// HandleTerminalConfig returns terminal session details including alt-screen state and subscriber counts.
func HandleTerminalConfig(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *Map) {
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
				info["subscribers"] = relay.fanout.Count()
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
func HandleTerminalUpload(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *Map) {
	if r.Method != "POST" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = "default"
	}

	// Cap request body at the upload limit (+4KB for overhead) to prevent
	// unbounded memory buffering before pty.Upload's own LimitReader kicks in.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20+4096)

	// Verify session exists.
	if _, err := mgr.Get(sessionID); err != nil {
		deps.JSONResponse(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	relay := relays.Get(sessionID)
	workspaceDir := ""
	if relay != nil {
		workspaceDir = relay.workspaceDir
	}
	if workspaceDir == "" {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "no workspace directory for session"})
		return
	}

	contentType := r.Header.Get("Content-Type")
	filename := r.URL.Query().Get("filename")

	result, err := pty.Upload(workspaceDir, sessionID, contentType, filename, r.Body)
	if err != nil {
		status := http.StatusBadRequest
		if err == pty.ErrUploadTooLarge {
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
