//go:build !windows
// +build !windows

// isolation_unix.go -- Unix implementation of stdout duplication for bridge MCP transport isolation via dup2/dup3 syscalls.
// Why: Separates the MCP transport fd from Go's os.Stdout so stderr/log output cannot corrupt JSON-RPC framing.
// Docs: docs/features/feature/bridge-restart/index.md

package stdioisolate

import (
	"fmt"
	"os"
	"syscall"
)

func duplicateStdoutForTransport(stdout *os.File) (*os.File, error) {
	fd, err := syscall.Dup(int(stdout.Fd()))
	if err != nil {
		return nil, err
	}
	// Critical: do not leak MCP transport FD into daemon child processes.
	// If inherited, os/exec stdout pipes never close and clients can hang.
	syscall.CloseOnExec(fd)
	dup := os.NewFile(uintptr(fd), "mcp-transport")
	if dup == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("os.NewFile returned nil for duplicated stdout")
	}
	return dup, nil
}

func redirectProcessStdStreams(target *os.File) error {
	if err := dup2Compat(int(target.Fd()), int(os.Stdout.Fd())); err != nil {
		return err
	}
	if err := dup2Compat(int(target.Fd()), int(os.Stderr.Fd())); err != nil {
		return err
	}
	return nil
}
