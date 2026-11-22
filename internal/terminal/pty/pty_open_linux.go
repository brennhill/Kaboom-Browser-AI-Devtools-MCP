// pty_open_linux.go — Linux PTY open/unlock/name resolution helpers.
// Why: Linux uses TIOCSPTLCK + TIOCGPTN ioctls for /dev/ptmx slave resolution.
// Docs: docs/features/feature/terminal/index.md

//go:build linux

package pty

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// openPTY opens a new PTY master/slave pair. Returns (master, slavePath, error).
func openPTY() (*os.File, string, error) {
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/ptmx: %w", err)
	}

	fd := ptmx.Fd()
	var unlock int32 = 0
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCSPTLCK),
		uintptr(unsafe.Pointer(&unlock)),
	); errno != 0 {
		_ = ptmx.Close()
		return nil, "", fmt.Errorf("unlockpt: %w", errno)
	}

	var ptyNum uint32
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCGPTN),
		uintptr(unsafe.Pointer(&ptyNum)),
	); errno != 0 {
		_ = ptmx.Close()
		return nil, "", fmt.Errorf("ptsnum: %w", errno)
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", ptyNum)
	return ptmx, slavePath, nil
}
