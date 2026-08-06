//go:build !windows
// +build !windows

// resume_unix.go — Sends SIGCONT to a suspended daemon after bridge reconnection.
// Why: Allows the bridge to wake a stopped daemon without requiring a full restart.
// Docs: docs/features/feature/bridge-restart/index.md

package processsignal

import (
	"os"
	"syscall"
)

// Resume wakes a suspended daemon process. A missing process is already gone.
func Resume(p *os.Process) {
	if p == nil {
		return
	}
	_ = p.Signal(syscall.SIGCONT)
}
