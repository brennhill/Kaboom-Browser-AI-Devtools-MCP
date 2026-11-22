// Purpose: Tests for bridge I/O isolation and file descriptor management.
// Docs: docs/features/feature/mcp-persistent-server/index.md

//go:build !windows

package stdioisolate

import (
	"os"
	"syscall"
	"testing"
)

func TestDuplicateStdoutForTransportSetsCloseOnExec(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer func() {
		_ = r.Close()
		_ = w.Close()
	}()

	dup, err := duplicateStdoutForTransport(w)
	if err != nil {
		t.Fatalf("duplicateStdoutForTransport() error = %v", err)
	}
	defer func() { _ = dup.Close() }()

	flagsRaw, _, errno := syscall.Syscall(syscall.SYS_FCNTL, dup.Fd(), syscall.F_GETFD, 0)
	if errno != 0 {
		t.Fatalf("fcntl(F_GETFD) error = %v", errno)
	}
	flags := int(flagsRaw)
	if flags&syscall.FD_CLOEXEC == 0 {
		t.Fatalf("duplicated transport fd missing FD_CLOEXEC: flags=%#x", flags)
	}
}
