// pty_open_unsupported.go — Fallback PTY opener for unsupported platforms.
// Why: Keeps cross-platform builds compiling when PTY ioctls are unavailable.
// Docs: docs/features/feature/terminal/index.md

//go:build !darwin && !linux

package pty

import (
	"errors"
	"os"
)

// openPTY opens a PTY when supported by the target OS.
func openPTY() (*os.File, string, error) {
	return nil, "", errors.New("pty: unsupported platform")
}
