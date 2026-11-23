// proclock_unix.go — flock(2)-backed exclusive locking for Unix platforms.
// Why: flock locks are owned by the open file description and are dropped by the
// kernel when the process exits, which is exactly the lifetime a singleton needs.

//go:build !windows

package proclock

import (
	"errors"
	"os"
	"syscall"
)

func lockFile(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return nil
	}
	// EWOULDBLOCK (== EAGAIN on Linux/Darwin) is the "someone else holds it"
	// signal, not an error condition.
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return ErrLocked
	}
	return err
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
