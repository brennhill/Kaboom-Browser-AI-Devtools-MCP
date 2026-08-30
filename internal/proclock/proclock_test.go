// proclock_test.go — Proves the OS-level exclusive lock is mutually exclusive,
// survives no-clean-shutdown, and is released by the kernel on process death.

package proclock_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/proclock"
)

func TestAcquireGrantsLockOnFreePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	lock, err := proclock.Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() error = %v, want nil", err)
	}
	if lock == nil {
		t.Fatal("Acquire() returned nil lock without error")
	}
	t.Cleanup(func() { _ = lock.Release() })
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file was not created: %v", err)
	}
}

func TestSecondAcquireInSameProcessIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	first, err := proclock.Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := proclock.Acquire(path)
	if !errors.Is(err, proclock.ErrLocked) {
		t.Fatalf("second Acquire() error = %v, want ErrLocked", err)
	}
	if second != nil {
		t.Fatal("second Acquire() returned a lock while one was held")
	}
}

func TestReleaseAllowsHandoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	first, err := proclock.Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	second, err := proclock.Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() after Release() error = %v, want nil", err)
	}
	_ = second.Release()
}

func TestReleaseIsIdempotentAndNilSafe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	lock, err := proclock.Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("first Release() error = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("second Release() error = %v, want nil (idempotent)", err)
	}
	var nilLock *proclock.Lock
	if err := nilLock.Release(); err != nil {
		t.Fatalf("nil Release() error = %v, want nil", err)
	}
}

// The whole point of an OS lock: a process that dies WITHOUT cleanup must not
// leave the lock held. This is what removes stale-lock heuristics entirely.
func TestKernelReleasesLockWhenHolderIsKilled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")
	ready := filepath.Join(dir, "ready")

	helper := exec.Command(os.Args[0], "-test.run=TestHelperHoldsLock") // #nosec G204 -- re-execs this test binary
	helper.Env = append(os.Environ(), "PROCLOCK_HELPER=1", "PROCLOCK_PATH="+path, "PROCLOCK_READY="+ready)
	if err := helper.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _, _ = helper.Process.Wait() }()

	// Wait on a readiness FILE, never by probing the lock: an Acquire attempt
	// that succeeds would itself take the lock and make this test hold what it
	// is trying to observe.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper never signalled that it holds the lock")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Confirm the helper's lock actually excludes us before we kill it.
	if _, err := proclock.Acquire(path); !errors.Is(err, proclock.ErrLocked) {
		t.Fatalf("Acquire() while helper holds lock = %v, want ErrLocked", err)
	}

	// SIGKILL: no defer, no cleanup, no chance to remove the file.
	if err := helper.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	_, _ = helper.Process.Wait()

	deadline = time.Now().Add(10 * time.Second)
	for {
		lock, err := proclock.Acquire(path)
		if err == nil {
			_ = lock.Release()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("lock still held after holder was SIGKILLed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHelperHoldsLock(t *testing.T) {
	if os.Getenv("PROCLOCK_HELPER") != "1" {
		t.Skip("helper process only")
	}
	lock, err := proclock.Acquire(os.Getenv("PROCLOCK_PATH"))
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("PROCLOCK_READY"), []byte("held"), 0o600); err != nil {
		os.Exit(3)
	}
	_ = lock
	time.Sleep(60 * time.Second)
	os.Exit(0)
}
