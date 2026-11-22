//go:build windows
// +build windows

// resume_windows.go — Defines the no-op Windows daemon resume operation.
// Why: Provides a compile-time shim so bridge signal resume works cross-platform.
// Docs: docs/features/feature/bridge-restart/index.md

package processsignal

import "os"

// Resume is a no-op because Windows has no SIGCONT equivalent.
func Resume(_ *os.Process) {}
