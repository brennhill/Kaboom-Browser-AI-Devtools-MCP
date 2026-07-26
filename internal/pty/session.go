//go:build darwin || linux

// session.go — PTY session: spawns a CLI subprocess in a pseudo-terminal.
// Why: Isolates PTY lifecycle (open, I/O, resize, close) from session management and HTTP transport.
// Docs: docs/features/feature/terminal/index.md

package pty

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// maxScrollback is the maximum size of the terminal output scrollback buffer (256 KB).
const maxScrollback = 256 * 1024

// sessionReapWait bounds each of Close's two waits for the reaper goroutine: the
// first after SIGTERM + PTY close, the second after escalating to SIGKILL. A var
// (not const) so tests can shorten it and exercise the give-up path without a
// real multi-second wait — the same seam writeBufferCloseTimeout uses.
var sessionReapWait = 2 * time.Second

// Alt-screen escape sequences (e.g., vim, htop).
var (
	altScreenEnter = []byte("\x1b[?1049h")
	altScreenExit  = []byte("\x1b[?1049l")
)

// ScrollbackSentinel is appended after replaying scrollback to distinguish
// historical data from live output. Frontends use this as a visual separator.
const ScrollbackSentinel = "\x1b]133;REPLAY_END\x07"

// defaultInputIDMax is the default capacity for the input dedup deque.
const defaultInputIDMax = 256

// IdleConfig configures idle detection for a session. The callback fires when
// the session produces output and then goes quiet for the specified duration.
type IdleConfig struct {
	Timeout  time.Duration
	Callback func(sessionID string)
}

// Session wraps a PTY master + child process for interactive terminal I/O.
type Session struct {
	ID         string
	ptmx       *os.File
	cmd        *exec.Cmd
	mu         sync.Mutex
	closed     bool
	done       chan struct{} // closed before ptmx.Close to signal in-flight I/O
	scrollback []byte        // ring buffer of recent PTY output for reconnect replay
	scrollMu   sync.Mutex    // protects scrollback, altScreen, idle, lastOutputAt

	// Alt-screen tracking (protected by scrollMu).
	altScreen    bool
	lastOutputAt time.Time

	// Idle detection (protected by scrollMu).
	idleTimeout time.Duration
	idleCb      func(string)
	idleTimer   stoppableTimer

	// clk is the time source for idle detection + last-output/-input timestamps.
	// nil means the real clock (see clock()); tests inject a fake for determinism.
	clk clock

	// Input tracking (protected by mu).
	lastInputAt time.Time
	inputIDs    []string
	inputIDMax  int

	// Child process reaping (set by reaper goroutine started in Spawn).
	reaped   chan struct{} // closed when child process has been reaped
	exitCode int           // set before reaped is closed
}

// winsize matches the C struct winsize for TIOCSWINSZ.
type winsize struct {
	Row uint16
	Col uint16
	X   uint16 // unused pixel width
	Y   uint16 // unused pixel height
}

// ErrSessionClosed is returned when operating on a closed session.
var ErrSessionClosed = errors.New("pty: session closed")

// SpawnConfig configures a new PTY session.
type SpawnConfig struct {
	ID   string   // Session identifier.
	Cmd  string   // CLI binary (e.g., "claude", "bash").
	Args []string // CLI arguments.
	Dir  string   // Working directory.
	Env  []string // Extra environment variables (appended to os.Environ).
	Cols uint16   // Initial terminal columns (default 80).
	Rows uint16   // Initial terminal rows (default 24).
}

// Spawn creates a new PTY session, starting the given command.
func Spawn(cfg SpawnConfig) (*Session, error) {
	if cfg.Cmd == "" {
		return nil, errors.New("pty: cmd is required")
	}
	if cfg.Cols == 0 {
		cfg.Cols = 80
	}
	if cfg.Rows == 0 {
		cfg.Rows = 24
	}
	if cfg.ID == "" {
		cfg.ID = "default"
	}

	ptmx, slavePath, err := openPTY()
	if err != nil {
		return nil, err
	}

	// Open slave.
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		ptmx.Close()
		return nil, fmt.Errorf("open slave %s: %w", slavePath, err)
	}

	// Set initial terminal size on the slave fd (macOS requires slave, not master).
	ws := &winsize{Row: cfg.Rows, Col: cfg.Cols}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, slave.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(ws))); errno != 0 {
		slave.Close()
		ptmx.Close()
		return nil, fmt.Errorf("set winsize: %w", errno)
	}

	cmd := exec.Command(cfg.Cmd, cfg.Args...)
	cmd.Dir = cfg.Dir
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	// Build environment: inherit from parent, add TERM, append extras.
	env := os.Environ()
	env = append(env, "TERM=xterm-256color")
	env = append(env, cfg.Env...)
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		slave.Close()
		ptmx.Close()
		return nil, fmt.Errorf("start %s: %w", cfg.Cmd, err)
	}

	// Slave fd is now owned by the child process; close our copy.
	slave.Close()

	s := &Session{
		ID:         cfg.ID,
		ptmx:       ptmx,
		cmd:        cmd,
		done:       make(chan struct{}),
		inputIDMax: defaultInputIDMax,
		reaped:     make(chan struct{}),
		clk:        realClock{},
	}
	go s.reap() // lint:allow-bare-goroutine — sole reaper for child process, exits when child exits
	return s, nil
}

// reapHook, when non-nil, is invoked by reap before waiting. Tests use it to
// force a panic and prove the reaper's recover keeps `reaped` closed (so Close
// never deadlocks) and does not crash the process.
var reapHook func()

// reap waits for the child process, captures its exit code, and closes reaped.
// It recovers from any panic: leaving reaped unclosed would deadlock Close (which
// blocks on <-s.reaped after SIGKILL), and an unrecovered panic in this spawned
// goroutine would crash the whole daemon. `defer close(s.reaped)` is registered
// after the recover defer, so during a panic unwind it runs FIRST — reaped is
// always closed before the recover logs.
func (s *Session) reap() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[Kaboom] PANIC in PTY reaper (session %s): %v\n%s\n", s.ID, r, debug.Stack())
		}
	}()
	defer close(s.reaped)
	if reapHook != nil {
		reapHook()
	}
	_ = s.cmd.Wait()
	if s.cmd.ProcessState != nil {
		s.exitCode = s.cmd.ProcessState.ExitCode()
	}
}

// Read reads from the PTY master (child's stdout/stderr).
// After unlocking the mutex, Close() may close the fd before the syscall runs.
// If the read fails, we check the done channel: if closed, the fd was recycled
// and we return ErrSessionClosed instead of the raw OS error.
func (s *Session) Read(buf []byte) (int, error) {
	s.mu.Lock() // lint:manual-unlock — unlock before blocking I/O
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSessionClosed
	}
	ptmx := s.ptmx
	s.mu.Unlock()

	n, err := ptmx.Read(buf)
	if err != nil {
		select {
		case <-s.done:
			return 0, ErrSessionClosed
		default:
		}
	}
	return n, err
}

// Write writes to the PTY master (child's stdin).
// After unlocking the mutex, Close() may close the fd before the syscall runs.
// If the write fails, we check the done channel: if closed, the fd was recycled
// and we return ErrSessionClosed instead of the raw OS error.
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock() // lint:manual-unlock — unlock before blocking I/O
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSessionClosed
	}
	s.lastInputAt = s.clock().Now()
	ptmx := s.ptmx
	s.mu.Unlock()

	n, err := ptmx.Write(data)
	if err != nil {
		select {
		case <-s.done:
			return 0, ErrSessionClosed
		default:
		}
	}
	return n, err
}

// Resize changes the terminal dimensions.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock() // lint:manual-unlock — unlock before ioctl syscall
	if s.closed {
		s.mu.Unlock()
		return ErrSessionClosed
	}
	ptmx := s.ptmx
	s.mu.Unlock()

	ws := &winsize{Row: rows, Col: cols}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, ptmx.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(ws)))
	if errno != 0 {
		return fmt.Errorf("resize: %w", errno)
	}
	return nil
}

// Close terminates the child process and closes the PTY.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	// Signal in-flight Read/Write calls that the fd is about to close.
	// They check this channel on error to return ErrSessionClosed instead
	// of a raw OS error from a potentially recycled fd number.
	close(s.done)

	// Stop idle timer if running. Lock ordering: mu (held) → scrollMu (safe).
	s.scrollMu.Lock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	s.scrollMu.Unlock()

	// Signal the child to terminate.
	s.signalChild(syscall.SIGTERM)
	// Close PTY master — this also signals EOF to the child.
	err := s.ptmx.Close()

	// Wait for the reaper goroutine (started in Spawn) with a timeout. If
	// SIGTERM + PTY close aren't enough, escalate to SIGKILL. Without this
	// timeout the reaper blocks indefinitely and deadlocks the Manager.
	select {
	case <-s.reaped:
		// Child exited cleanly.
	case <-time.After(sessionReapWait):
		// Escalate to SIGKILL.
		s.signalChild(syscall.SIGKILL)
		// Bound the post-SIGKILL wait too. A child stuck in uninterruptible
		// (D-state) sleep may not be reaped promptly; without this bound, Close —
		// and StopAll during daemon shutdown — would block indefinitely. The
		// reaper goroutine closes s.reaped whenever the kernel finally reaps it.
		select {
		case <-s.reaped:
		case <-time.After(sessionReapWait):
			// Giving up here leaves a live process holding the slave PTY after the
			// daemon believes the session is gone. Nothing else observes this, so
			// it must not be silent (rule 25).
			diag(EventSessionReapTimeout, map[string]any{
				"session_id": s.ID,
				"pid":        s.Pid(),
				"wait_ms":    (2 * sessionReapWait).Milliseconds(),
			})
		}
	}
	return err
}

// signalChild sends sig to the child and reports a genuine failure. An
// already-exited child (os.ErrProcessDone) is the expected case on a clean exit,
// not a failure — logging it would bury the real signal failures (EPERM, a PID
// recycled out from under us) in noise. Caller holds s.mu.
func (s *Session) signalChild(sig syscall.Signal) {
	if s.cmd.Process == nil {
		return
	}
	err := s.cmd.Process.Signal(sig)
	if err == nil || errors.Is(err, os.ErrProcessDone) {
		return
	}
	diag(EventSessionSignalFailed, map[string]any{
		"session_id": s.ID,
		"signal":     sig.String(),
		"pid":        s.cmd.Process.Pid,
		"error":      err.Error(),
	})
}

// Wait blocks until the child process exits.
func (s *Session) Wait() error {
	<-s.reaped
	return nil
}

// AppendScrollback appends data to the scrollback buffer, evicting oldest bytes
// if the buffer exceeds maxScrollback. Also tracks alt-screen state and resets
// the idle timer.
func (s *Session) AppendScrollback(data []byte) {
	s.scrollMu.Lock() // lint:manual-unlock — simple lock/unlock in same function

	// Track alt-screen state — last sequence in the chunk wins.
	if enterIdx := bytes.LastIndex(data, altScreenEnter); enterIdx >= 0 {
		if exitIdx := bytes.LastIndex(data, altScreenExit); exitIdx > enterIdx {
			s.altScreen = false
		} else {
			s.altScreen = true
		}
	} else if bytes.Contains(data, altScreenExit) {
		s.altScreen = false
	}

	// Track output time and reset idle timer.
	s.lastOutputAt = s.clock().Now()
	if s.idleCb != nil && s.idleTimeout > 0 {
		if s.idleTimer != nil {
			s.idleTimer.Reset(s.idleTimeout)
		} else {
			id := s.ID
			cb := s.idleCb
			timeout := s.idleTimeout
			s.idleTimer = s.clock().AfterFunc(timeout, func() { cb(id) })
		}
	}

	s.scrollback = append(s.scrollback, data...)
	if len(s.scrollback) > maxScrollback {
		// Allocate a new slice to release the old backing array.
		// Sub-slicing (s.scrollback[len-max:]) retains a reference to the
		// original backing array, causing unbounded memory growth.
		trimmed := make([]byte, maxScrollback)
		copy(trimmed, s.scrollback[len(s.scrollback)-maxScrollback:])
		s.scrollback = trimmed
	}
	s.scrollMu.Unlock()
}

// Scrollback returns a copy of the current scrollback buffer.
func (s *Session) Scrollback() []byte {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	if len(s.scrollback) == 0 {
		return nil
	}
	out := make([]byte, len(s.scrollback))
	copy(out, s.scrollback)
	return out
}

// IsAlive returns true if the session is not closed and the child process is still running.
func (s *Session) IsAlive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.cmd.Process == nil {
		return false
	}
	// Signal 0 checks process existence without sending a signal.
	err := s.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// Pid returns the child process PID, or -1 if not started.
func (s *Session) Pid() int {
	if s.cmd.Process == nil {
		return -1
	}
	return s.cmd.Process.Pid
}

// ForceRedraw sends SIGWINCH to the child process, forcing TUI applications
// to redraw. Used on WebSocket reconnect where the terminal dimensions may
// not have changed (so TIOCSWINSZ alone wouldn't trigger SIGWINCH).
func (s *Session) ForceRedraw() {
	s.mu.Lock() // lint:manual-unlock — unlock before signal
	if s.closed || s.cmd.Process == nil {
		s.mu.Unlock()
		return
	}
	proc := s.cmd.Process
	s.mu.Unlock()
	_ = proc.Signal(syscall.SIGWINCH)
}

// AltScreenActive returns true if the terminal is in alt-screen mode
// (e.g., vim, htop). The frontend uses this to toggle scrollbar visibility
// and copy-paste behavior.
func (s *Session) AltScreenActive() bool {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	return s.altScreen
}

// SetIdleConfig configures idle detection. The callback fires when no output
// is produced for the given timeout duration after the most recent output.
// Pass zero Timeout to disable.
func (s *Session) SetIdleConfig(cfg IdleConfig) {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	s.idleTimeout = cfg.Timeout
	s.idleCb = cfg.Callback
}

// LastOutputAt returns the last time PTY output was received.
func (s *Session) LastOutputAt() time.Time {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	return s.lastOutputAt
}

// LastInputAt returns the last time input was written to the session.
func (s *Session) LastInputAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastInputAt
}

// clock returns the session's time source, defaulting to the real clock when
// unset so a zero-value Session (used in some unit tests) still works.
func (s *Session) clock() clock {
	if s.clk == nil {
		return realClock{}
	}
	return s.clk
}

// ScrollbackWithSentinel returns the scrollback buffer followed by a sentinel
// marker. Subscribers use the sentinel to distinguish replayed history from
// live data on reconnect.
func (s *Session) ScrollbackWithSentinel() []byte {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	if len(s.scrollback) == 0 {
		return []byte(ScrollbackSentinel)
	}
	out := make([]byte, len(s.scrollback)+len(ScrollbackSentinel))
	copy(out, s.scrollback)
	copy(out[len(s.scrollback):], ScrollbackSentinel)
	return out
}

// ExitCode returns the child process exit code, or -1 if the process hasn't
// been reaped yet. Non-blocking — returns immediately.
func (s *Session) ExitCode() int {
	select {
	case <-s.reaped:
		return s.exitCode
	default:
		return -1
	}
}

// WriteWithID writes to the PTY with input deduplication. If the inputID has
// been seen recently, the write is silently dropped (returns 0, nil). This
// prevents duplicate keystrokes on WebSocket reconnect/retry.
func (s *Session) WriteWithID(data []byte, inputID string) (int, error) {
	if inputID == "" {
		return s.Write(data)
	}
	s.mu.Lock() // lint:manual-unlock — unlock before blocking I/O
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSessionClosed
	}
	for _, id := range s.inputIDs {
		if id == inputID {
			s.mu.Unlock()
			return 0, nil
		}
	}
	s.inputIDs = append(s.inputIDs, inputID)
	if len(s.inputIDs) > s.inputIDMax {
		s.inputIDs = s.inputIDs[1:]
	}
	s.lastInputAt = s.clock().Now()
	ptmx := s.ptmx
	s.mu.Unlock()

	n, err := ptmx.Write(data)
	if err != nil {
		select {
		case <-s.done:
			return 0, ErrSessionClosed
		default:
		}
	}
	return n, err
}
