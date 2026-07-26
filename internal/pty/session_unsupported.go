//go:build !darwin && !linux

// session_unsupported.go — PTY session stubs for unsupported platforms.
// Why: Keep non-Unix targets (for example, Windows) compiling without PTY ioctl support.
// Docs: docs/features/feature/terminal/index.md

package pty

import (
	"errors"
	"time"
)

// maxScrollback keeps API parity with Unix session implementation.
const maxScrollback = 256 * 1024

// defaultInputIDMax keeps API parity with Unix session implementation.
const defaultInputIDMax = 256

// ScrollbackSentinel keeps API parity with Unix session implementation.
const ScrollbackSentinel = "\x1b]133;REPLAY_END\x07"

// IdleConfig configures idle detection for a session.
type IdleConfig struct {
	Timeout  time.Duration
	Callback func(sessionID string)
}

// Session is a stub PTY session on unsupported platforms.
type Session struct {
	ID string
}

// SpawnConfig configures a new PTY session.
type SpawnConfig struct {
	ID   string
	Cmd  string
	Args []string
	Dir  string
	Env  []string
	Cols uint16
	Rows uint16
}

// ErrSessionClosed is returned when operating on a closed session.
var ErrSessionClosed = errors.New("pty: session closed")

// ErrReapTimeout keeps API parity with the Unix session implementation.
var ErrReapTimeout = errors.New("pty: timed out waiting for the child to be reaped")

var errUnsupportedPTY = errors.New("pty: unsupported platform")

// Spawn returns unsupported on non-Unix platforms.
func Spawn(cfg SpawnConfig) (*Session, error) {
	return nil, errUnsupportedPTY
}

// Read is unsupported on non-Unix platforms.
func (s *Session) Read(buf []byte) (int, error) {
	return 0, errUnsupportedPTY
}

// Write is unsupported on non-Unix platforms.
func (s *Session) Write(data []byte) (int, error) {
	return 0, errUnsupportedPTY
}

// Resize is unsupported on non-Unix platforms.
func (s *Session) Resize(cols, rows uint16) error {
	return errUnsupportedPTY
}

// Close is a no-op for unsupported platform stubs.
func (s *Session) Close() error {
	return nil
}

// Wait is a no-op for unsupported platform stubs.
func (s *Session) Wait(timeout time.Duration) error {
	return nil
}

// AppendScrollback is a no-op for unsupported platform stubs.
func (s *Session) AppendScrollback(data []byte) {}

// Scrollback returns an empty buffer on unsupported platforms.
func (s *Session) Scrollback() []byte {
	return nil
}

// IsAlive reports false on unsupported platforms.
func (s *Session) IsAlive() bool {
	return false
}

// Pid returns 0 on unsupported platforms.
func (s *Session) Pid() int {
	return 0
}

// ForceRedraw is a no-op on unsupported platforms.
func (s *Session) ForceRedraw() {}

// AltScreenActive reports false on unsupported platforms.
func (s *Session) AltScreenActive() bool { return false }

// SetIdleConfig is a no-op on unsupported platforms.
func (s *Session) SetIdleConfig(cfg IdleConfig) {}

// LastOutputAt returns zero time on unsupported platforms.
func (s *Session) LastOutputAt() time.Time { return time.Time{} }

// LastInputAt returns zero time on unsupported platforms.
func (s *Session) LastInputAt() time.Time { return time.Time{} }

// ScrollbackWithSentinel returns sentinel only on unsupported platforms.
func (s *Session) ScrollbackWithSentinel() []byte { return []byte(ScrollbackSentinel) }

// ExitCode returns -1 on unsupported platforms.
func (s *Session) ExitCode() int { return -1 }

// WriteWithID is unsupported on non-Unix platforms.
func (s *Session) WriteWithID(data []byte, inputID string) (int, error) {
	return 0, errUnsupportedPTY
}
