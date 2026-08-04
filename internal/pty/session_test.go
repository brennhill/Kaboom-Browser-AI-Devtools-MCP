// session_test.go — Tests for PTY session spawn, I/O roundtrip, resize, and cleanup.

package pty

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockingPTY struct {
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockingPTY(entries int) *blockingPTY {
	return &blockingPTY{entered: make(chan struct{}, entries), closed: make(chan struct{})}
}

func (pty *blockingPTY) block() (int, error) {
	pty.entered <- struct{}{}
	<-pty.closed
	return 0, os.ErrClosed
}

func (pty *blockingPTY) Read([]byte) (int, error)  { return pty.block() }
func (pty *blockingPTY) Write([]byte) (int, error) { return pty.block() }
func (pty *blockingPTY) Close() error {
	pty.once.Do(func() { close(pty.closed) })
	return nil
}
func (pty *blockingPTY) Fd() uintptr { return 0 }

func sessionWithBlockingPTY(pty *blockingPTY) *Session {
	return &Session{ptmx: pty, cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: closedChan()}
}

func readUntilContains(t *testing.T, sess *Session, needle string, timeout time.Duration) string {
	t.Helper()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(timeout)
	var collected string
	for time.Now().Before(deadline) {
		n, err := sess.Read(buf)
		if n > 0 {
			collected += string(buf[:n])
			if strings.Contains(collected, needle) {
				return collected
			}
		}
		if err != nil {
			t.Fatalf("read before %q: %v (output so far: %q)", needle, err, collected)
		}
	}
	t.Fatalf("timeout waiting for %q, got: %q", needle, collected)
	return ""
}

func TestSpawn_RequiresCmd(t *testing.T) {
	_, err := Spawn(SpawnConfig{})
	if err == nil {
		t.Fatal("expected error for empty cmd")
	}
	if !strings.Contains(err.Error(), "cmd is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawn_InvalidCmd(t *testing.T) {
	_, err := Spawn(SpawnConfig{Cmd: "/nonexistent/binary"})
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestSpawn_EchoRoundtrip(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "echo HELLO_PTY; exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	// Read output — should contain "HELLO_PTY".
	readUntilContains(t, sess, "HELLO_PTY", 3*time.Second)

	// Write to stdin (cat echoes back).
	input := "test_input\n"
	if _, err := sess.Write([]byte(input)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the echo back.
	readUntilContains(t, sess, "test_input", 3*time.Second)
}

func TestSpawn_DefaultSize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "stty size; exit 0"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	collected := readUntilContains(t, sess, "24 80", 3*time.Second)
	if !strings.Contains(collected, "24 80") {
		t.Fatalf("expected '24 80' in output, got: %q", collected)
	}
}

func TestSpawn_CustomSize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "stty size; exit 0"},
		Cols: 120,
		Rows: 40,
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	collected := readUntilContains(t, sess, "40 120", 3*time.Second)
	if !strings.Contains(collected, "40 120") {
		t.Fatalf("expected '40 120' in output, got: %q", collected)
	}
}

func TestSession_Resize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	if err := sess.Resize(132, 50); err != nil {
		t.Fatalf("resize: %v", err)
	}
}

func TestSession_Close(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Operations after close should return ErrSessionClosed.
	buf := make([]byte, 64)
	if _, err := sess.Read(buf); err != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed on Read, got: %v", err)
	}
	if _, err := sess.Write([]byte("x")); err != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed on Write, got: %v", err)
	}
	if err := sess.Resize(80, 24); err != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed on Resize, got: %v", err)
	}

	// Double close is safe.
	if err := sess.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestSession_IsAlive(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Running session should be alive.
	if !sess.IsAlive() {
		t.Fatal("expected IsAlive()=true for running session")
	}

	// After close, should not be alive.
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if sess.IsAlive() {
		t.Fatal("expected IsAlive()=false after Close()")
	}
}

func TestSession_Pid(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	pid := sess.Pid()
	if pid <= 0 {
		t.Fatalf("expected positive PID, got: %d", pid)
	}
}

// T1 regression: Read/Write during concurrent Close must return ErrSessionClosed,
// not a raw OS error from a potentially recycled file descriptor.
func TestSession_ReadDuringClose_ReturnsErrSessionClosed(t *testing.T) {
	pty := newBlockingPTY(1)
	sess := sessionWithBlockingPTY(pty)

	// Start a goroutine that reads forever; it will unblock when Close() closes the fd.
	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := sess.Read(buf)
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	<-pty.entered

	// Close the session — the read should unblock with ErrSessionClosed.
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case err := <-readErr:
		if err != ErrSessionClosed {
			t.Fatalf("expected ErrSessionClosed from in-flight Read, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for Read to return after Close")
	}
}

func TestSession_WriteDuringClose_ReturnsErrSessionClosed(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Close the session first.
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Write after close must return ErrSessionClosed.
	_, werr := sess.Write([]byte("hello"))
	if werr != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed from Write after Close, got: %v", werr)
	}
}

// T1 regression: done channel is closed before ptmx, so concurrent callers
// detect shutdown via the channel rather than hitting a recycled fd.
func TestSession_DoneChannelClosedBeforePtmx(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Before close, done channel should not be closed.
	select {
	case <-sess.done:
		t.Fatal("done channel should not be closed before Close()")
	default:
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// After close, done channel must be closed.
	select {
	case <-sess.done:
		// Good — channel is closed.
	default:
		t.Fatal("done channel should be closed after Close()")
	}
}

// T3 regression: scrollback trimming must not leak memory via backing array retention.
// Verifies that after eviction, the scrollback slice has its own backing array
// (length == cap == maxScrollback), not a sub-slice of a larger allocation.
func TestSession_AppendScrollback_EvictionReleasesBackingArray(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	// Fill scrollback past maxScrollback to trigger eviction.
	chunk := make([]byte, 1024)
	for i := range chunk {
		chunk[i] = 'A'
	}
	// Write 2x maxScrollback worth of data to force at least one eviction.
	iterations := (maxScrollback * 2) / len(chunk)
	for i := 0; i < iterations; i++ {
		sess.AppendScrollback(chunk)
	}

	sess.scrollMu.Lock()
	sbLen := len(sess.scrollback)
	sbCap := cap(sess.scrollback)
	sess.scrollMu.Unlock()

	if sbLen != maxScrollback {
		t.Fatalf("expected scrollback len=%d, got %d", maxScrollback, sbLen)
	}
	// After eviction with make+copy, cap should equal len (no excess backing array).
	if sbCap != maxScrollback {
		t.Fatalf("expected scrollback cap=%d (new allocation), got %d (backing array leak)", maxScrollback, sbCap)
	}
}

// T3 regression: scrollback below maxScrollback should not trigger eviction.
func TestSession_AppendScrollback_NoEvictionBelowMax(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	data := []byte("hello")
	sess.AppendScrollback(data)

	got := sess.Scrollback()
	if string(got) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(got))
	}
}

// T1 regression: concurrent Read/Write/Close must not panic or deadlock.
func TestSession_ConcurrentReadWriteClose(t *testing.T) {
	const goroutines = 10
	pty := newBlockingPTY(2 * goroutines)
	sess := sessionWithBlockingPTY(pty)
	var wg sync.WaitGroup

	// Spawn readers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 64)
			_, _ = sess.Read(buf)
		}()
	}

	// Spawn writers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sess.Write([]byte("x"))
		}()
	}

	for i := 0; i < 2*goroutines; i++ {
		<-pty.entered
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// All goroutines must exit within a reasonable time.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines exited.
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: concurrent Read/Write goroutines did not exit after Close")
	}
}

// --- Alt-screen detection (improvement 4) ---

func TestSession_AltScreenDetection(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	if sess.AltScreenActive() {
		t.Fatal("expected alt-screen inactive initially")
	}

	sess.AppendScrollback([]byte("some output\x1b[?1049h"))
	if !sess.AltScreenActive() {
		t.Fatal("expected alt-screen active after enter sequence")
	}

	sess.AppendScrollback([]byte("\x1b[?1049l"))
	if sess.AltScreenActive() {
		t.Fatal("expected alt-screen inactive after exit sequence")
	}
}

func TestSession_AltScreen_LastSequenceWins(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	// Both sequences in one chunk — exit is last.
	sess.AppendScrollback([]byte("\x1b[?1049h some data \x1b[?1049l"))
	if sess.AltScreenActive() {
		t.Fatal("expected alt-screen inactive when exit is last")
	}

	// Enter is last.
	sess.AppendScrollback([]byte("\x1b[?1049l some data \x1b[?1049h"))
	if !sess.AltScreenActive() {
		t.Fatal("expected alt-screen active when enter is last")
	}
}

// --- Scrollback replay sentinel (improvement 3) ---

func TestSession_ScrollbackWithSentinel(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	sess.AppendScrollback([]byte("previous output"))
	result := sess.ScrollbackWithSentinel()
	expected := "previous output" + ScrollbackSentinel
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}

func TestSession_ScrollbackWithSentinel_Empty(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	result := sess.ScrollbackWithSentinel()
	if string(result) != ScrollbackSentinel {
		t.Fatalf("expected sentinel only, got %q", string(result))
	}
}

// --- Input deduplication (improvement 6) ---

func TestSession_WriteWithID_Dedup(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	n, err := sess.WriteWithID([]byte("hello\n"), "msg-1")
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if n == 0 {
		t.Fatal("expected non-zero bytes written")
	}

	// Duplicate should be silently dropped.
	n, err = sess.WriteWithID([]byte("hello\n"), "msg-1")
	if err != nil {
		t.Fatalf("dup write: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes for duplicate, got %d", n)
	}

	// Different ID should succeed.
	n, err = sess.WriteWithID([]byte("world\n"), "msg-2")
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if n == 0 {
		t.Fatal("expected non-zero bytes for new ID")
	}
}

func TestSession_WriteWithID_EmptyID(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	n, err := sess.WriteWithID([]byte("x"), "")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 byte, got %d", n)
	}
}

func TestSession_WriteWithID_BoundedDeque(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	for i := 0; i < defaultInputIDMax+10; i++ {
		_, err := sess.WriteWithID([]byte("x"), fmt.Sprintf("id-%d", i))
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	// Oldest IDs should have been evicted and can be reused.
	n, err := sess.WriteWithID([]byte("x"), "id-0")
	if err != nil {
		t.Fatalf("reuse write: %v", err)
	}
	if n == 0 {
		t.Fatal("expected evicted ID to be accepted again")
	}
}

// --- Timestamp tracking ---

func TestSession_LastOutputAt(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	if !sess.LastOutputAt().IsZero() {
		t.Fatal("expected zero LastOutputAt initially")
	}

	before := time.Now()
	sess.AppendScrollback([]byte("data"))
	after := time.Now()

	ts := sess.LastOutputAt()
	if ts.Before(before) || ts.After(after) {
		t.Fatalf("LastOutputAt %v not between %v and %v", ts, before, after)
	}
}

func TestSession_LastInputAt(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	if !sess.LastInputAt().IsZero() {
		t.Fatal("expected zero LastInputAt initially")
	}

	before := time.Now()
	sess.Write([]byte("x"))
	after := time.Now()

	ts := sess.LastInputAt()
	if ts.Before(before) || ts.After(after) {
		t.Fatalf("LastInputAt %v not between %v and %v", ts, before, after)
	}
}
