// proclock.go — OS-level exclusive process locks held for a process's lifetime.
// Why: A singleton enforced by lock FILES needs stale detection, PID liveness
// probes, and grace windows, and still races (read-decide-write is not atomic).
// A lock held by the KERNEL cannot race and is released automatically when the
// holder dies — including SIGKILL, OOM, and panic — so none of that is needed.
// Docs: docs/core/reliability/zombie-prevention.md

package proclock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrLocked reports that another process already holds the lock. It is a normal
// outcome (someone else won the election), never a failure.
var ErrLocked = errors.New("lock is held by another process")

// Lock is an acquired exclusive lock. The underlying descriptor is held open for
// as long as the lock is: closing it is what releases the kernel lock, so callers
// must keep the Lock alive for the duration they need exclusivity.
type Lock struct {
	mu     sync.Mutex
	file   *os.File
	path   string
	closed bool
}

// Path reports the lock file this Lock holds.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Acquire takes an exclusive, non-blocking lock on path, creating the file and
// its parent directory if needed. It returns ErrLocked when another process holds
// it. The lock is released by Release or by this process exiting for any reason.
func Acquire(path string) (*Lock, error) {
	if path == "" {
		return nil, errors.New("proclock: empty lock path")
	}
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("proclock: create lock directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- path is a caller-owned runtime state file
	if err != nil {
		return nil, fmt.Errorf("proclock: open lock file %s: %w", path, err)
	}
	if err := lockFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, ErrLocked) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("proclock: lock %s: %w", path, err)
	}
	return &Lock{file: file, path: path}, nil
}

// Release drops the lock. It is safe to call on a nil Lock and safe to call more
// than once — a second call is a no-op, not an error.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.file == nil {
		return nil
	}
	l.closed = true
	// Closing the descriptor releases the kernel lock. Unlocking first keeps the
	// release explicit on platforms where close-implies-unlock is not documented.
	unlockErr := unlockFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("proclock: unlock %s: %w", l.path, unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("proclock: close %s: %w", l.path, closeErr)
	}
	return nil
}

// Write replaces the lock file's contents with data. Holding the lock and owning
// its payload in one file lets a winner publish who it is (pid, port, version)
// without a second file that could disagree with the lock.
func (l *Lock) Write(data []byte) error {
	if l == nil {
		return errors.New("proclock: write on nil lock")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.file == nil {
		return errors.New("proclock: write on released lock")
	}
	if err := l.file.Truncate(0); err != nil {
		return fmt.Errorf("proclock: truncate %s: %w", l.path, err)
	}
	if _, err := l.file.WriteAt(data, 0); err != nil {
		return fmt.Errorf("proclock: write %s: %w", l.path, err)
	}
	return l.file.Sync()
}

// ReadUnlocked reads a lock file's payload without acquiring it. Used by census
// and diagnostics to report who holds a lock; it must never gate behavior, since
// the payload can be mid-write while the holder is perfectly healthy.
func ReadUnlocked(path string) ([]byte, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is a caller-owned runtime state file
	if err != nil {
		return nil, err
	}
	return data, nil
}
