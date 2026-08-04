// relay.go -- Per-session relay: fan-out PTY output, buffer writes, prompt detection.
// Why: Supports multiple WebSocket viewers per session and non-blocking input.

package terminal

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Relay manages per-session fan-out, buffered writes, and a PTY reader loop.
// The reader loop runs from relay creation until the session closes.
type Relay struct {
	sess         *pty.Session
	fanout       *pty.Fanout
	writeBuf     *pty.WriteBuffer
	workspaceDir string
	done         chan struct{}
	exitCode     int         // set by readLoop before closing fanout; read by downstream after channel close
	ended        atomic.Bool // set true by readLoop before fanout.Close(); distinguishes session-end from a slow-subscriber drop
	// subMu serializes the readLoop's append+broadcast (appendAndBroadcast) against
	// SubscribeWithHistory's snapshot+subscribe, so a reconnecting viewer's history
	// snapshot and channel registration never straddle a chunk — every chunk lands
	// in exactly one of the two (no duplicate, no gap) at the reconnect boundary.
	subMu sync.Mutex
	// onExit, when set, runs once readLoop has finished tearing the relay down. The
	// Map uses it to drop a relay whose session ended on its own; without it the
	// entry (plus its fanout and write buffer) lingered until an explicit
	// /terminal/stop that may never come.
	onExit func(*Relay)
}

// NewRelay creates a relay and starts the PTY reader loop.
func NewRelay(sess *pty.Session, workspaceDir string) *Relay {
	r := newRelay(sess, workspaceDir, nil)
	r.start()
	return r
}

// newRelay builds a relay WITHOUT starting its reader loop, so the Map can publish
// it before the loop (which may exit and self-remove immediately) can run. Callers
// must call start exactly once.
func newRelay(sess *pty.Session, workspaceDir string, onExit func(*Relay)) *Relay {
	return &Relay{
		sess:         sess,
		fanout:       pty.NewFanout(),
		writeBuf:     pty.NewWriteBuffer(sess),
		workspaceDir: workspaceDir,
		done:         make(chan struct{}),
		onExit:       onExit,
	}
}

// start launches the PTY reader loop.
func (r *Relay) start() { util.SafeGo(r.readLoop) }

// readLoop continuously reads PTY output, appends to scrollback, and broadcasts
// to all subscribers. Exits when the session closes or the process exits.
// Before closing the fanout, it reaps the child process to capture the exit code
// so downstream subscribers can notify the browser.
func (r *Relay) readLoop() {
	// Defers unwind bottom-up: write buffer, then fanout, then r.done, then the
	// Map's self-removal — so by the time onExit runs (and by the time Close's wait
	// on r.done returns) the relay is fully torn down.
	defer close(r.done)
	defer func() {
		if r.onExit != nil {
			r.onExit(r)
		}
	}()
	defer r.fanout.Close()
	defer r.writeBuf.Close()
	buf := make([]byte, ReadBufSize)
	for {
		n, err := r.sess.Read(buf)
		if n > 0 {
			r.appendAndBroadcast(buf[:n])
		}
		if err != nil {
			// Reap child process to capture exit code before fanout closes.
			// The write to exitCode happens-before fanout.Close() (in defers),
			// which closes subscriber channels, creating a happens-before edge
			// to the downstream goroutine's read of exitCode.
			r.reapExitCode()
			// Mark the session genuinely ended BEFORE the deferred fanout.Close()
			// closes subscriber channels. A subscriber channel also closes when
			// Fanout.Broadcast drops a slow subscriber (backpressure) while the
			// session is still alive; this flag lets the downstream pump tell the
			// two apart so it does not falsely report `exited` on a live shell.
			r.ended.Store(true)
			return
		}
	}
}

// appendAndBroadcast records a PTY output chunk to scrollback and fans it out to
// subscribers as ONE step under subMu, so it is atomic against
// SubscribeWithHistory's snapshot+subscribe. Without this, the append (scrollMu)
// and the broadcast (fanout mutex) are two separately-locked operations a
// concurrent reconnect could straddle, replaying a chunk that is also broadcast
// (duplicate) or dropping one that is neither (lost). Both callers copy the data,
// so the caller's reusable read buffer is safe to pass.
func (r *Relay) appendAndBroadcast(data []byte) {
	r.subMu.Lock()
	r.sess.AppendScrollback(data)
	r.fanout.Broadcast(data)
	r.subMu.Unlock()
}

// SubscribeWithHistory snapshots the session's scrollback and registers a fanout
// subscriber atomically under subMu (the same lock appendAndBroadcast holds), so
// the reconnect boundary between replayed history and live channel output is
// seamless: every chunk is in exactly one of the two. Returns the same errors as
// Fanout.Subscribe — ErrFanoutClosed (session ended), ErrFanoutFull (cap), or
// ErrDuplicateSubscriber (a reused subID; WS ids come from NextWSSubID and are
// unique by construction).
func (r *Relay) SubscribeWithHistory(subID string) (history []byte, sub <-chan []byte, err error) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	history = r.sess.Scrollback()
	sub, err = r.fanout.Subscribe(subID)
	return history, sub, err
}

// reapExitCode waits (bounded) for the child process exit code. Called after PTY
// read returns an error (typically EOF when the child exits), so the Session's
// reaper goroutine has usually already captured the exit code.
//
// The bound matters: this runs BEFORE readLoop's deferred fanout.Close(), so an
// unreapable child would otherwise park readLoop forever — the fanout never
// closes and every WebSocket pump hangs on a channel that never closes (S6). On
// timeout, ExitCode() reports -1 (unknown) and teardown proceeds; the session
// layer logs the give-up.
func (r *Relay) reapExitCode() {
	_ = r.sess.Wait(ReapTimeout)
	r.exitCode = r.sess.ExitCode()
}

// Close tears the relay down and waits (bounded) for that teardown to finish.
//
// It used to close only the write buffer, which stopped nothing: readLoop stayed
// blocked in sess.Read, so the goroutine, the PTY fd and the open fanout all
// survived a /terminal/stop or a daemon shutdown (finding S5). Closing the session
// is what actually breaks readLoop out — its Read returns ErrSessionClosed and its
// defers close the fanout and write buffer and then r.done.
//
// Every step is idempotent (Session.Close, WriteBuffer.Close and Fanout.Close all
// no-op when already closed), so Close is safe to call twice and safe to call
// concurrently with readLoop's own teardown. In production the session is normally
// already stopped by the time we get here (Manager.Stop in /terminal/stop, StopAll
// at shutdown), which makes the session close a no-op and r.done already closed.
func (r *Relay) Close() {
	_ = r.sess.Close()

	select {
	case <-r.done:
	case <-time.After(RelayCloseTimeout):
		// readLoop is wedged (an unreapable child, or a write buffer whose drain is
		// stuck). Do not block the caller — shutdown and /terminal/stop must still
		// make progress; the session-layer diagnostics record the underlying stall.
	}

	// Defense in depth: if the wait above timed out, readLoop has not run its
	// defers, so at least stop accepting writes that can never be delivered.
	r.writeBuf.Close()
}

// Fanout returns the relay's fanout for subscribing.
func (r *Relay) Fanout() *pty.Fanout { return r.fanout }

// WriteBuf returns the relay's write buffer for writing to the PTY.
func (r *Relay) WriteBuf() *pty.WriteBuffer { return r.writeBuf }

// WorkspaceDir returns the workspace directory for this relay.
func (r *Relay) WorkspaceDir() string { return r.workspaceDir }

// ExitCode returns the exit code captured after the session exits.
func (r *Relay) ExitCode() int { return r.exitCode }

// Ended reports whether the session genuinely ended (readLoop exited), as opposed
// to a subscriber channel closing because Fanout dropped a slow subscriber. Safe
// to call from another goroutine after observing a subscriber-channel close: the
// happens-before edge is through that channel close (readLoop stores ended before
// the deferred fanout.Close()).
func (r *Relay) Ended() bool { return r.ended.Load() }

// Map manages per-session relays. Implements RelayMap.
type Map struct {
	mu     sync.Mutex
	relays map[string]*Relay
}

// NewMap creates a new relay map.
func NewMap() *Map {
	return &Map{relays: make(map[string]*Relay)}
}

// Get returns the relay for the given session ID, or nil.
func (m *Map) Get(id string) *Relay {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.relays[id]
}

// GetOrCreate returns the existing relay for id, or creates one. If an existing
// relay is bound to a DIFFERENT session than the caller supplied, it is stale and
// gets replaced rather than returned.
//
// That rebind is the fix for the permanent fake "exited": nothing removes a relay
// when its shell dies, so the map keeps an entry whose fanout is closed. Manager.Start
// evicts a dead session before spawning and discards `replaced` if the spawn then
// fails, so a later successful Start reports Replaced=false and the caller lands
// here — and this function used to ignore `sess` entirely and hand back the stale
// dead-bound relay. Every WS connect then hit ErrFanoutClosed and shipped
// {"type":"exited"}, which permanently disables reconnect in the browser: the panel
// showed "[Process exited]" over a healthy shell until the daemon restarted.
//
// Keeping the invariant here rather than in the handler is deliberate (rule 19):
// no caller should have to compute `Replaced` correctly for the relay map to stay
// consistent with the session manager.
func (m *Map) GetOrCreate(id string, sess *pty.Session, workspaceDir string) *Relay {
	m.mu.Lock() // lint:manual-unlock — unlock before Close to avoid holding lock during I/O
	if r := m.relays[id]; r != nil {
		// Rebind ONLY when the bound session is genuinely DEAD. The condition must be
		// asymmetric: "the sessions differ" is symmetric, so a concurrent reconnect
		// still holding the OLD session would rebind the map backwards and undo a
		// just-completed ReplaceRelay — finding H in reverse. A corpse can never win
		// that race, because a dead session is never something to rebind *to*.
		if r.sess == sess || r.sess == nil || r.sess.IsAlive() {
			m.mu.Unlock()
			return r
		}
		fresh := NewRelay(sess, workspaceDir)
		m.relays[id] = fresh
		m.mu.Unlock()
		r.Close() // outside the lock: Close can block briefly
		return fresh
	}
	fresh := m.installLocked(id, sess, workspaceDir)
	m.mu.Unlock()
	return fresh
}

// installLocked registers a fresh relay under id and only THEN starts its reader
// loop. Order matters: the loop can exit immediately (a session that is already
// dead) and self-remove, and a self-removal that ran before the registration would
// leave the dead relay in the map forever. Caller holds m.mu.
func (m *Map) installLocked(id string, sess *pty.Session, workspaceDir string) *Relay {
	r := newRelay(sess, workspaceDir, func(dead *Relay) { m.removeIfCurrent(id, dead) })
	m.relays[id] = r
	r.start()
	return r
}

// removeIfCurrent drops id's entry only when it still points at r. A relay that
// has already been replaced (ReplaceRelay) or removed must not evict its
// successor when its own readLoop finally unwinds.
func (m *Map) removeIfCurrent(id string, r *Relay) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.relays[id] == r {
		delete(m.relays, id)
	}
}

// ReplaceRelay atomically drops any existing relay for id and installs a fresh one
// bound to sess, under a single lock hold. The self-heal path (Start replacing a
// dead session) must not do this as a separate Remove + GetOrCreate: a concurrent
// GetOrCreate (a WS reconnect that fetched the session before the token was
// invalidated) could slip into the gap and bind the relay to the just-evicted dead
// session, which GetOrCreate would then return unchanged. Overwriting
// unconditionally under one lock guarantees the fresh binding wins regardless of
// interleaving (finding H). The old relay is Closed outside the lock (Close can
// block briefly) like Remove.
func (m *Map) ReplaceRelay(id string, sess *pty.Session, workspaceDir string) *Relay {
	m.mu.Lock() // lint:manual-unlock — unlock before Close to avoid holding lock during I/O
	old := m.relays[id]
	r := m.installLocked(id, sess, workspaceDir)
	m.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return r
}

// Remove stops and removes the relay for the given session ID.
func (m *Map) Remove(id string) {
	m.mu.Lock() // lint:manual-unlock — unlock before Close to avoid holding lock during I/O
	r := m.relays[id]
	delete(m.relays, id)
	m.mu.Unlock()
	if r != nil {
		r.Close()
	}
}

// WriteToFirst writes data to the first active relay's PTY input.
// Assumes a single active terminal session (the typical case). If multiple
// sessions exist, the target is non-deterministic due to Go map iteration.
// Returns true if a relay was found and the write succeeded.
func (m *Map) WriteToFirst(data []byte) bool {
	m.mu.Lock()
	var relay *Relay
	for _, r := range m.relays {
		relay = r
		break
	}
	m.mu.Unlock()
	if relay == nil {
		return false
	}
	if _, err := relay.writeBuf.Write(data); err != nil {
		// ErrWriteBufferClosed (the shell exited) and ErrWriteBufferFull (wedged
		// under backpressure) are distinct, but both mean the data did NOT land.
		// Report failure so the caller falls through to its fallback (e.g. the
		// in-page Audit prompt stores the intent) instead of being told the write
		// succeeded and silently losing it.
		return false
	}
	return true
}

// CloseAll stops and removes all relays.
func (m *Map) CloseAll() {
	m.mu.Lock() // lint:manual-unlock — unlock before Close to avoid holding lock during I/O
	toClose := make([]*Relay, 0, len(m.relays))
	for _, r := range m.relays {
		toClose = append(toClose, r)
	}
	m.relays = make(map[string]*Relay)
	m.mu.Unlock()
	for _, r := range toClose {
		r.Close()
	}
}

// wsSubCounter generates unique subscriber IDs for WebSocket connections.
var wsSubCounter atomic.Uint64

// NextWSSubID returns a unique subscriber ID for WebSocket connections.
func NextWSSubID() string {
	return fmt.Sprintf("ws-%d", wsSubCounter.Add(1))
}

// WaitForPromptViaRelay subscribes to the relay's fan-out, watches for a
// shell prompt character, then writes the init command. Replaces the old
// direct-PTY-read approach so the relay's readLoop owns all PTY reads.
func WaitForPromptViaRelay(relay *Relay, initCmd string) {
	// A UNIQUE id per call: a constant "init-cmd" makes two concurrent inits on one
	// relay collide (finding I). Fanout.Subscribe now rejects a duplicate id outright
	// (ErrDuplicateSubscriber) instead of silently replacing the incumbent, so the
	// collision can no longer orphan a subscriber — but the second init would still
	// fall through to the blind write below, so the unique id stays.
	subID := NextWSSubID()
	ch, err := relay.fanout.Subscribe(subID)
	if err != nil {
		_, _ = relay.writeBuf.Write([]byte(initCmd + "\n"))
		return
	}
	defer relay.fanout.Unsubscribe(subID)

	deadline := time.After(InitTimeout)
	for {
		select {
		case <-deadline:
			_, _ = relay.writeBuf.Write([]byte(initCmd + "\n"))
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			for _, b := range data {
				if strings.ContainsRune(PromptChars, rune(b)) {
					_, _ = relay.writeBuf.Write([]byte(initCmd + "\n"))
					return
				}
			}
		}
	}
}
