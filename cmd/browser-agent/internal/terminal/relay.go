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
}

// NewRelay creates a relay and starts the PTY reader loop.
func NewRelay(sess *pty.Session, workspaceDir string) *Relay {
	r := &Relay{
		sess:         sess,
		fanout:       pty.NewFanout(),
		writeBuf:     pty.NewWriteBuffer(sess),
		workspaceDir: workspaceDir,
		done:         make(chan struct{}),
	}
	util.SafeGo(r.readLoop)
	return r
}

// readLoop continuously reads PTY output, appends to scrollback, and broadcasts
// to all subscribers. Exits when the session closes or the process exits.
// Before closing the fanout, it reaps the child process to capture the exit code
// so downstream subscribers can notify the browser.
func (r *Relay) readLoop() {
	defer close(r.done)
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
// Fanout.Subscribe — ErrFanoutClosed (session ended) or ErrFanoutFull (cap).
func (r *Relay) SubscribeWithHistory(subID string) (history []byte, sub <-chan []byte, err error) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	history = r.sess.Scrollback()
	sub, err = r.fanout.Subscribe(subID)
	return history, sub, err
}

// reapExitCode waits for the child process exit code. Called after PTY read
// returns an error (typically EOF when the child exits), so the Session's
// reaper goroutine has usually already captured the exit code.
func (r *Relay) reapExitCode() {
	r.sess.Wait() // blocks until child exits — usually instant since PTY EOF already received
	r.exitCode = r.sess.ExitCode()
}

// Close stops the write buffer. The readLoop exits when the session closes,
// which triggers fanout and writeBuf cleanup via defers.
func (r *Relay) Close() {
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

// GetOrCreate returns the existing relay for id or creates a new one.
func (m *Map) GetOrCreate(id string, sess *pty.Session, workspaceDir string) *Relay {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.relays[id]; r != nil {
		return r
	}
	r := NewRelay(sess, workspaceDir)
	m.relays[id] = r
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
		// A full/closed write buffer means the shell has exited or is wedged under
		// backpressure — the data did NOT land. Report failure so the caller falls
		// through to its fallback (e.g. the in-page Audit prompt stores the intent)
		// instead of being told the write succeeded and silently losing it.
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
	subID := "init-cmd"
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
