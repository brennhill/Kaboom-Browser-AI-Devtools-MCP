//go:build darwin || linux

// session_reaper_test.go — Regression test: the child-reaper goroutine must
// survive a panic without crashing the daemon and without leaving `reaped`
// unclosed (which would deadlock Session.Close).

package pty

import (
	"os/exec"
	"testing"
)

// TestSession_ReaperRecoversPanic proves that if the reaper panics, the panic
// is recovered (does not escape and crash the process) and `reaped` is still
// closed so Close() — which blocks on <-s.reaped — cannot deadlock.
func TestSession_ReaperRecoversPanic(t *testing.T) {
	reapHook = func() { panic("reaper boom") }
	defer func() { reapHook = nil }()

	s := &Session{ID: "reaper-panic", reaped: make(chan struct{}), cmd: &exec.Cmd{}}

	// Call reap synchronously. An escaped panic would crash the test binary;
	// reaching the assertions below proves the recover held.
	s.reap()

	select {
	case <-s.reaped:
		// Required: closed despite the panic.
	default:
		t.Fatal("reaper must close reaped even when it panics")
	}
}
