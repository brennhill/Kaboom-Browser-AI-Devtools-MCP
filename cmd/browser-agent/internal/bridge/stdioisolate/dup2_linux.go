//go:build linux
// +build linux

// dup2_linux.go -- Provides Linux-specific dup3 syscall wrapper for file descriptor duplication in bridge IO isolation.
// Why: Linux requires dup3 instead of dup2 for safe fd duplication with close-on-exec semantics.
// Docs: docs/features/feature/bridge-restart/index.md

package stdioisolate

import "syscall"

func dup2Compat(oldfd, newfd int) error {
	if oldfd == newfd {
		return nil
	}
	return syscall.Dup3(oldfd, newfd, 0)
}
