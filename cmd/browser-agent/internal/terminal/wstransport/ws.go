// ws.go -- WebSocket transport for the in-browser terminal.
// Why: The upgrade handshake, the three-goroutine relay loop, and the shared
// deadline-bound frame writer are one cohesive transport concern, separable from
// session lifecycle and static asset serving. Split out of handlers.go to keep both
// files within the 800-line limit.
// Docs: docs/features/feature/terminal/index.md

package wstransport

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/dimensions"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	ptyfanout "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty/fanout"
)

const (
	// PingInterval is how often the server probes an idle connection.
	PingInterval = 30 * time.Second
	// PongTimeout bounds time without any incoming frame.
	PongTimeout = 60 * time.Second
	// WriteTimeout bounds a single WebSocket frame write.
	WriteTimeout = 10 * time.Second
)

// Deps are the explicit HTTP, codec, and diagnostic collaborators for transport.
type Deps struct {
	JSONResponse func(http.ResponseWriter, int, any)
	Stderrf      func(string, ...any)
	LogEvent     func(string, map[string]any)
	WSReadFrame  func(io.Reader) (bool, byte, []byte, error)
	WSWriteFrame func(*bufio.ReadWriter, byte, []byte) error
	WSAcceptKey  func(string) string
}

func (d Deps) logEvent(event string, fields map[string]any) {
	if d.LogEvent != nil {
		d.LogEvent(event, fields)
	}
}

// HandleTerminalWS upgrades a GET /terminal/ws request to a WebSocket connection
// that relays raw PTY I/O to/from the browser's xterm.js terminal emulator.
func Handle(w http.ResponseWriter, r *http.Request, deps Deps, mgr *pty.Manager, relays *sessionrelay.Map) {
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

	// Build the deadline-aware frame writer immediately after the handshake and use
	// it for EVERY write on this connection — the pre-wsLoop replay chunks,
	// replay_end, and subscribe-failure frames, as well as wsLoop's own frames
	// (threaded in below). The terminal server runs with WriteTimeout:0, so without
	// this a client that stalls its reader mid-replay would block conn.Write forever
	// and leak a goroutine+fd per connect (finding B). One writer is shared so all
	// callers serialize on the same mutex.
	writeFrame := NewFrameWriter(conn, bufrw, deps)

	// Get or create relay for multi-subscriber fan-out.
	relay := relays.GetOrCreate(sess.ID, sess, "")

	// Snapshot scrollback and register the subscriber ATOMICALLY (one subMu hold in
	// the relay) against the readLoop's append+broadcast. A plain two-step
	// (Scrollback then Subscribe) lets a chunk arriving in the gap be both replayed
	// AND broadcast (duplicate) or neither (lost) at the reconnect boundary — the
	// readLoop appends under scrollMu then broadcasts under the fanout mutex, two
	// separate locks this snapshot+subscribe would otherwise straddle (finding C).
	subID := sessionrelay.NextWSSubID()
	history, sub, subErr := relay.SubscribeWithHistory(subID)

	// Replay scrollback so the reconnecting (or first-connecting) terminal sees prior output.
	if len(history) > 0 {
		for off := 0; off < len(history); off += sessionrelay.BufferSize {
			end := off + sessionrelay.BufferSize
			if end > len(history) {
				end = len(history)
			}
			if err := writeFrame(0x2, history[off:end]); err != nil {
				if subErr == nil {
					relay.Fanout().Unsubscribe(subID)
				}
				_ = conn.Close()
				return
			}
		}
	}
	replayEnd, _ := json.Marshal(map[string]string{"type": "replay_end"})
	if err := writeFrame(0x1, replayEnd); err != nil {
		if subErr == nil {
			relay.Fanout().Unsubscribe(subID)
		}
		_ = conn.Close()
		return
	}

	// If subscribe failed, distinguish the two causes (Fanout.Subscribe returns
	// both): ErrFanoutClosed means the shell already exited before this reconnect —
	// send the final scrollback, an `exited` notification, and close, so the browser
	// shows the death and stops reconnecting. ErrFanoutFull means the 32-subscriber
	// cap was hit while the shell is HEALTHY — send a plain close (no `exited`) so
	// the client's reconnect backoff retries instead of declaring a live terminal
	// dead (finding A).
	if subErr != nil {
		if errors.Is(subErr, ptyfanout.ErrFanoutClosed) {
			exitMsg, _ := json.Marshal(map[string]any{"type": "exited", "code": relay.ExitCode()})
			_ = writeFrame(0x1, exitMsg)
		}
		_ = writeFrame(0x8, nil)
		_ = conn.Close()
		return
	}

	deps.logEvent("terminal_ws_connect", map[string]any{"session_id": sess.ID, "sub_id": subID})
	wsLoop(conn, bufrw, deps, sess, relay, sub, writeFrame)
	relay.Fanout().Unsubscribe(subID)
	deps.logEvent("terminal_ws_disconnect", map[string]any{"session_id": sess.ID, "sub_id": subID})
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
// writeFrame is the shared, deadline-bound frame writer created in
// HandleTerminalWS right after the handshake and threaded through here, so the
// pre-loop replay writes and wsLoop's downstream/ping/control writes all serialize
// on ONE mutex (a second NewFrameWriter would double-serialize on a different
// mutex and defeat the shared-writer invariant).
func wsLoop(conn net.Conn, rw *bufio.ReadWriter, deps Deps, sess *pty.Session, relay *sessionrelay.Relay, sub <-chan []byte, writeFrame func(opcode byte, payload []byte) error) {
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

	// Fan-out -> WebSocket (downstream): read from subscriber channel and send as binary frames.
	// Also tracks alt-screen state changes and notifies the frontend.
	downstreamDone := make(chan struct{})
	goConnWorker(deps, sess.ID, "downstream", closeConn, func() {
		defer close(downstreamDone)
		prevAltScreen := sess.AltScreenActive()
		for {
			select {
			case data, ok := <-sub:
				if !ok {
					if relay.Ended() {
						// Session genuinely ended — send exit notification so the
						// browser can display the message and stop reconnecting.
						exitMsg, _ := json.Marshal(map[string]any{"type": "exited", "code": relay.ExitCode()})
						_ = writeFrame(0x1, exitMsg)
					} else {
						// This subscriber was dropped for being slow (fanout
						// backpressure) while the shell is still running. Do NOT
						// declare it `exited` — just close so the browser reconnects
						// and replays scrollback instead of showing a dead terminal.
						deps.logEvent("terminal_ws_subscriber_dropped", map[string]any{"session_id": sess.ID})
					}
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
	})

	// Server-initiated ping keepalive — detects dead connections (browser crash,
	// laptop sleep) without ever timing out idle users.
	pingTicker := time.NewTicker(PingInterval)
	goConnWorker(deps, sess.ID, "ping", closeConn, func() {
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
				// Also send an application-level keepalive the BROWSER can observe.
				// The ping above is a WebSocket control frame: the browser answers it
				// automatically but never surfaces it to JS, so page code has no way
				// to tell a live-but-idle terminal from a half-open socket (suspend,
				// NAT rebind — no FIN/RST, readyState stays OPEN). Without an
				// observable signal the client shows a connected dot while keystrokes
				// vanish. A text frame is visible to onmessage, so the client can run
				// its own liveness watchdog. Non-fatal: the ping already governs
				// connection health, so a failed keepalive must not tear down a
				// connection the ping considers fine.
				_ = writeFrame(0x1, []byte(`{"type":"keepalive"}`))
			}
		}
	})

	// WebSocket -> PTY (upstream): read frames and dispatch.
	// Uses relay.writeBuf for non-blocking writes with backpressure.
	// NOTE: Do NOT call sess.Close() on WebSocket disconnect — the session
	// must survive page refreshes so the browser can reconnect with scrollback replay.
	// Sessions are only killed explicitly via POST /terminal/stop (the Exit button).
	goConnWorker(deps, sess.ID, "upstream", closeConn, func() {
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
				if _, werr := relay.WriteBuf().Write(payload); werr != nil {
					// The keystrokes did NOT reach the shell. Dropping them silently
					// is exactly the "typed input vanished" report we cannot diagnose
					// afterwards (rule 25), and the two causes need different
					// responses, so the reason is recorded, not just the drop.
					deps.logEvent("terminal_input_dropped", map[string]any{
						"session_id": sess.ID,
						"bytes":      len(payload),
						"reason":     writeDropReason(werr),
					})
				}
			case 0x1: // Text — JSON control message
				HandleControlMessage(payload, sess)
			}
		}
	})

	<-downstreamDone
}

// writeDropReason names why a PTY write was refused, so the log distinguishes a
// session that has ended from one that is merely wedged under backpressure
// (finding S9 — both used to surface as ErrWriteBufferFull).
func writeDropReason(err error) string {
	switch {
	case errors.Is(err, pty.ErrWriteBufferClosed):
		return "session_ended"
	case errors.Is(err, pty.ErrWriteBufferFull):
		return "backpressure"
	default:
		return "write_error"
	}
}

// goConnWorker launches one per-connection wsLoop goroutine with panic recovery.
// The daemon invariant is that nothing it is sent may crash the process: a panic
// in the downstream pump, ping keepalive, or upstream reader (e.g. a write on a
// closed pipe, a malformed frame, a nil deref on hostile input) must tear down
// ONLY this connection. On panic it logs a structured, session-correlated event
// (so the outage is diagnosable in ~/.kaboom/logs/kaboom.jsonl) and calls
// closeConn so the sibling goroutines unwind and the browser can reconnect with
// scrollback replay. fn's own deferred cleanup (e.g. close(downstreamDone)) still
// runs during unwind because it is registered inside fn, below this recover.
func goConnWorker(deps Deps, sessionID, role string, closeConn func(), fn func()) {
	go func() { // lint:allow-bare-goroutine — recover-wrapped; bounded by connDone/channel close
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				deps.logEvent("terminal_ws_panic", map[string]any{
					"session_id": sessionID,
					"role":       role,
					"panic":      fmt.Sprintf("%v", r),
					"stack":      string(stack),
				})
				if deps.Stderrf != nil {
					deps.Stderrf("[Kaboom] PANIC in terminal WS %s goroutine (session %s): %v\n%s\n", role, sessionID, r, stack)
				}
				closeConn()
			}
		}()
		fn()
	}()
}

// writeDeadliner is the subset of net.Conn that NewFrameWriter uses to bound
// frame writes. Nil in tests that write to an in-memory buffer.
type writeDeadliner interface {
	SetWriteDeadline(time.Time) error
}

// NewFrameWriter returns a thread-safe frame writer for one WebSocket
// connection. All callers for that connection must share this writer. When conn
// is non-nil, each write is bounded by WSWriteTimeout so a stalled reader cannot
// wedge the downstream/ping goroutines.
func NewFrameWriter(conn writeDeadliner, rw *bufio.ReadWriter, deps Deps) func(opcode byte, payload []byte) error {
	var wsWriteMu sync.Mutex
	return func(opcode byte, payload []byte) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		if conn != nil {
			// Refresh the deadline per frame: a slow-but-progressing connection
			// keeps getting a fresh window; only a truly stalled write errors,
			// and the caller tears the connection down on that error.
			_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		}
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
			cols, rows, ok := dimensions.Resolve(msg.Cols, msg.Rows)
			if !ok {
				return
			}
			_ = sess.Resize(cols, rows)
			// Always force SIGWINCH so TUI apps redraw — TIOCSWINSZ only
			// sends SIGWINCH when dimensions actually change, but on reconnect
			// the dimensions may match while the display is stale.
			sess.ForceRedraw()
		}
	}
}
