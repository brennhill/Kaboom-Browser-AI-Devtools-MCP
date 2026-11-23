// pty_open_darwin.go — Darwin PTY open/grant/unlock/name resolution helpers.
// Why: macOS requires TIOCPTY* ioctls to unlock the slave PTY and resolve slave path.
// Docs: docs/features/feature/terminal/index.md

//go:build darwin

package pty

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// ptsname returns the slave PTY path for a master fd.
// On macOS, TIOCPTYGNAME writes a null-terminated C string into a 128-byte buffer.
func ptsname(f *os.File) (string, error) {
	buf := make([]byte, 128)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return "", fmt.Errorf("ptsname: %w", errno)
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}

// openPTY opens a new PTY master/slave pair. Returns (master, slavePath, error).
func openPTY() (*os.File, string, error) {
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/ptmx: %w", err)
	}

	fd := ptmx.Fd()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCPTYGRANT, 0); errno != 0 {
		_ = ptmx.Close()
		return nil, "", fmt.Errorf("grantpt: %w", errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCPTYUNLK, 0); errno != 0 {
		_ = ptmx.Close()
		return nil, "", fmt.Errorf("unlockpt: %w", errno)
	}

	slavePath, err := ptsname(ptmx)
	if err != nil {
		_ = ptmx.Close()
		return nil, "", err
	}

	return ptmx, slavePath, nil
}
