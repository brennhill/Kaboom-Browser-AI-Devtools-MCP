// lock_test.go — Deterministic startup leadership and ownership tests.
package startuplock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestAcquireElectsOneLeaderAndReleaseAllowsHandoff(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	manager := NewManager("0.9.0", func(int) bool { return true })
	first, acquired, err := manager.Acquire(7890)
	if err != nil || !acquired || first == nil {
		t.Fatalf("first Acquire() = (%v, %v, %v), want lock, true, nil", first, acquired, err)
	}
	if second, acquired, err := manager.Acquire(7890); err != nil || acquired || second != nil {
		t.Fatalf("second Acquire() = (%v, %v, %v), want nil, false, nil", second, acquired, err)
	}
	first.Release()
	third, acquired, err := manager.Acquire(7890)
	if err != nil || !acquired || third == nil {
		t.Fatalf("handoff Acquire() = (%v, %v, %v), want lock, true, nil", third, acquired, err)
	}
	third.Release()
}

func TestReleaseCannotDeleteAnotherOwnersClaim(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	path, err := Path(7907)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(Record{PID: 999999, Port: 7907, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	(&Lock{path: path, pid: 12345}).Release()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("another owner's claim was removed: %v", err)
	}
	var nilLock *Lock
	nilLock.Release()
	(&Lock{}).Release()
}

func TestClearStaleUsesControlledClockAndLiveness(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	manager := NewManager("test", func(pid int) bool { return pid == 42 })
	manager.Now = func() time.Time { return now }
	manager.PID = func() int { return 42 }
	lock, acquired, err := manager.Acquire(7908)
	if err != nil || !acquired {
		t.Fatalf("Acquire() = (%v, %v, %v)", lock, acquired, err)
	}
	if manager.ClearStale(7908, time.Minute) {
		t.Fatal("fresh live claim was classified stale")
	}
	manager.Now = func() time.Time { return now.Add(2 * time.Minute) }
	if !manager.ClearStale(7908, time.Minute) {
		t.Fatal("expired claim was not cleared")
	}
}
