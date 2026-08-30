// proclock_windows.go — LockFileEx-backed exclusive locking for Windows.
// Why: Windows has no flock. LockFileEx with LOCKFILE_EXCLUSIVE_LOCK and
// LOCKFILE_FAIL_IMMEDIATELY gives the same semantics, and the lock is dropped by
// the kernel when the owning handle closes — including on abnormal termination.
// Bound through syscall's lazy DLL loader so the zero-dependency rule holds.

//go:build windows

package proclock

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	errLockViolation        = syscall.Errno(33)
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	procLockEx   = kernel32.NewProc("LockFileEx")
	procUnlockEx = kernel32.NewProc("UnlockFileEx")
)

func lockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	ret, _, err := procLockEx.Call(
		file.Fd(),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0,
		^uintptr(0),
		^uintptr(0),
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret != 0 {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errLockViolation {
		return ErrLocked
	}
	return err
}

func unlockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	ret, _, err := procUnlockEx.Call(
		file.Fd(),
		0,
		^uintptr(0),
		^uintptr(0),
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret != 0 {
		return nil
	}
	return err
}
