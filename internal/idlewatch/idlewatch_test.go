// idlewatch_test.go — Proves a daemon with no work exits, a daemon with work never
// does, and that a test daemon cannot outlive its run.

package idlewatch_test

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/idlewatch"
)

func at(base time.Time, d time.Duration) time.Time { return base.Add(d) }

func TestIdleDaemonExitsAfterTheIdleWindow(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	watcher := idlewatch.New(idlewatch.Config{
		IdleAfter: 10 * time.Minute,
		Busy:      func() (bool, string) { return false, "" },
	}, base)

	if exit, _ := watcher.Tick(at(base, 9*time.Minute)); exit {
		t.Fatal("Tick() asked to exit before the idle window elapsed")
	}
	exit, reason := watcher.Tick(at(base, 10*time.Minute+time.Second))
	if !exit {
		t.Fatal("Tick() did not ask to exit after the idle window elapsed")
	}
	if reason == "" {
		t.Error("Tick() returned no reason for exiting")
	}
}

// The guarantee that keeps a working session alive: any activity resets the clock,
// so a daemon in use is never shut down under a developer.
func TestBusyDaemonNeverExits(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	busy := true
	watcher := idlewatch.New(idlewatch.Config{
		IdleAfter: 10 * time.Minute,
		Busy:      func() (bool, string) { return busy, "a client is connected" },
	}, base)

	for minute := 1; minute <= 60; minute++ {
		if exit, reason := watcher.Tick(at(base, time.Duration(minute)*time.Minute)); exit {
			t.Fatalf("Tick() at minute %d asked a busy daemon to exit: %s", minute, reason)
		}
	}
	// Work stops; only now does the window start running.
	busy = false
	if exit, _ := watcher.Tick(at(base, 65*time.Minute)); exit {
		t.Fatal("Tick() exited immediately after work stopped; the window must restart")
	}
	if exit, _ := watcher.Tick(at(base, 76*time.Minute)); !exit {
		t.Fatal("Tick() did not exit ten minutes after work stopped")
	}
}

func TestActivityResetsAPartiallyElapsedWindow(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	busy := false
	watcher := idlewatch.New(idlewatch.Config{
		IdleAfter: 10 * time.Minute,
		Busy:      func() (bool, string) { return busy, "" },
	}, base)

	watcher.Tick(at(base, 9*time.Minute)) // 9 minutes idle
	busy = true
	watcher.Tick(at(base, 9*time.Minute+30*time.Second)) // activity resets
	busy = false
	if exit, _ := watcher.Tick(at(base, 15*time.Minute)); exit {
		t.Fatal("Tick() exited on a window that activity had reset")
	}
}

// A --parallel daemon belongs to one test run. If that run dies without stopping
// it, a hard lifetime bound is the only thing that reclaims its two ports.
func TestMaxLifetimeExitsEvenWhileBusy(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	watcher := idlewatch.New(idlewatch.Config{
		IdleAfter:   time.Hour,
		MaxLifetime: 5 * time.Minute,
		Busy:        func() (bool, string) { return true, "still serving" },
	}, base)

	if exit, _ := watcher.Tick(at(base, 4*time.Minute)); exit {
		t.Fatal("Tick() exceeded lifetime early")
	}
	exit, reason := watcher.Tick(at(base, 5*time.Minute+time.Second))
	if !exit {
		t.Fatal("Tick() did not enforce the maximum lifetime on a busy test daemon")
	}
	if reason == "" {
		t.Error("Tick() gave no reason for the lifetime exit")
	}
}

func TestZeroMaxLifetimeMeansUnlimited(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	watcher := idlewatch.New(idlewatch.Config{
		IdleAfter:   time.Hour,
		MaxLifetime: 0,
		Busy:        func() (bool, string) { return true, "serving" },
	}, base)
	if exit, _ := watcher.Tick(at(base, 30*24*time.Hour)); exit {
		t.Fatal("Tick() enforced a lifetime when MaxLifetime was 0")
	}
}

// A zero idle window must not mean "exit immediately" -- that would make a
// misconfigured daemon unusable. It means the idle check is disabled.
func TestZeroIdleAfterDisablesIdleExit(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	watcher := idlewatch.New(idlewatch.Config{
		IdleAfter: 0,
		Busy:      func() (bool, string) { return false, "" },
	}, base)
	if exit, _ := watcher.Tick(at(base, 365*24*time.Hour)); exit {
		t.Fatal("Tick() exited with idle checking disabled")
	}
}

// If we cannot tell whether the daemon is busy, we must assume it is. Exiting on
// an unknown state would kill a daemon mid-recording.
func TestNilBusyProbeIsTreatedAsBusy(t *testing.T) {
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	watcher := idlewatch.New(idlewatch.Config{IdleAfter: time.Minute, Busy: nil}, base)
	if exit, _ := watcher.Tick(at(base, time.Hour)); exit {
		t.Fatal("Tick() exited without a way to check for work in progress")
	}
}
