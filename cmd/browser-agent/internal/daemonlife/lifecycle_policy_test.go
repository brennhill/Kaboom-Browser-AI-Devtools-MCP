// Purpose: In-package tests for daemonlife's public entry points — the startup
// policy gate and lock-file ownership — driven entirely through injected Deps.
// The counterpart wiring test in package main proves the host binds these seams
// to real primitives; this one proves the policy itself.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package daemonlife

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

func TestDaemonLockCanonicalFaultsAreRedactedAndFailSafe(t *testing.T) {
	for _, kind := range []statefault.Kind{statefault.Read, statefault.Write, statefault.Quota, statefault.Cancellation} {
		t.Run(string(kind), func(t *testing.T) {
			isolatedStateDir(t)
			scenario := statefault.New(kind, "private-daemon-lock")
			files := faultLifecycleFilesystem{}
			if kind == statefault.Read {
				files.readErr = scenario.Error()
			} else {
				files.writeErr = scenario.Error()
			}
			installLifecycleFault(t, files)
			diagnostics := statediag.NewCollector()

			if kind == statefault.Read {
				deps, logger := newTestDeps(t)
				deps.Recovery = diagnostics
				if err := EnforceStartupPolicy(deps, 7890, LaunchOptions{}); err != nil {
					t.Fatalf("read fault should recover: %v", err)
				}
				if event := logger.find("daemon_lock_recovered"); event == nil || event.Fields["error"] != nil {
					t.Fatalf("recovery event = %#v, want redacted reason", event)
				}
			} else if err := PersistCurrentLock(7890, "0.9.0", diagnostics); err == nil {
				t.Fatal("write fault must prevent unsafe ownership claim")
			}
			if got := diagnostics.Snapshot(); len(got) == 0 || got[0].Name != "daemon_lock_state" {
				t.Fatalf("diagnostics = %#v, want daemon lock incident", got)
			}
		})
	}
}

// isolatedStateDir points the whole package at a temp state root for one test.
func isolatedStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(state.StateDirEnv, dir)
	return dir
}

// seedLock writes a lock record through the package's own writer.
func seedLock(t *testing.T, rec daemonLockRecord) {
	t.Helper()
	if err := writeDaemonLockFile(rec); err != nil {
		t.Fatalf("writeDaemonLockFile() error = %v", err)
	}
}

func TestEnforceStartupPolicy_NoLockFile_Proceeds(t *testing.T) {
	isolatedStateDir(t)
	deps, _ := newTestDeps(t)
	if err := EnforceStartupPolicy(deps, 7890, LaunchOptions{}); err != nil {
		t.Fatalf("no lock file should permit startup, got %v", err)
	}
}

func TestEnforceStartupPolicy_CorruptLockFileRecoversAndReports(t *testing.T) {
	isolatedStateDir(t)
	path, err := daemonLockFilePath()
	if err != nil {
		t.Fatalf("daemonLockFilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	deps, _ := newTestDeps(t)
	diagnostics := statediag.NewCollector()
	deps.Recovery = diagnostics
	err = EnforceStartupPolicy(deps, 7890, LaunchOptions{})
	if err != nil {
		t.Fatalf("corrupt lock must not block startup: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("stale lock still exists: %v", statErr)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "daemon_lock_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable daemon-lock warning", got)
	}
}

func TestEnforceStartupPolicy_OwnLockIsANoOp(t *testing.T) {
	dir := isolatedStateDir(t)
	seedLock(t, daemonLockRecord{PID: os.Getpid(), Port: 7890, StateDir: dir, Version: "0.8.8"})

	deps, _ := newTestDeps(t)
	deps.IsProcessAlive = func(int) bool {
		t.Fatal("our own PID must be recognized before any liveness probe")
		return false
	}
	if err := EnforceStartupPolicy(deps, 7890, LaunchOptions{}); err != nil {
		t.Fatalf("a lock owned by this process should be a no-op, got %v", err)
	}
}

// TestEnforceStartupPolicy_DeadOwnerClearsLock pins the dead-owner FAST PATH
// specifically. Ownership is made to match (ReadPIDFile == lock PID) so the
// stale-mismatch branch cannot fire, and every downstream effect is trapped:
// a dead owner must be reclaimed without a health probe, an HTTP shutdown, or a
// signal. Asserting only "the lock is gone" is not enough — several other
// branches also end in a removed lock, so such a test stays green even if the
// liveness check is deleted entirely.
func TestEnforceStartupPolicy_DeadOwnerClearsLock(t *testing.T) {
	dir := isolatedStateDir(t)
	const deadPID = 999001
	seedLock(t, daemonLockRecord{PID: deadPID, Port: 7890, StateDir: dir, Version: "0.8.8"})

	deps, log := newTestDeps(t)
	deps.IsProcessAlive = func(int) bool { return false }
	deps.ReadPIDFile = func(int) int { return deadPID } // ownership proven, not a mismatch
	deps.TerminatePID = func(int, bool) { t.Fatal("a dead owner must never be signalled") }
	deps.TryShutdown = func(int) bool { t.Fatal("a dead owner must never be asked to shut down"); return false }
	deps.FetchHealth = func(context.Context, int, time.Duration) (bool, string, bool) {
		t.Fatal("a dead owner must not be health-probed")
		return false, "", false
	}

	if err := EnforceStartupPolicy(deps, 7890, LaunchOptions{}); err != nil {
		t.Fatalf("a dead owner's lock should be reclaimed, got %v", err)
	}
	if rec, err := readDaemonLockFile(); err != nil || rec != nil {
		t.Fatalf("stale lock should be gone, got rec=%v err=%v", rec, err)
	}
	// The reclaim must be attributed to the dead owner, not to a PID mismatch.
	if evt := log.find("daemon_lock_reclaimed_stale_mismatch"); evt != nil {
		t.Fatalf("dead owner should take the liveness fast path, not the mismatch path: %+v", *evt)
	}
}

func TestEnforceStartupPolicy_InvalidLockMetadata(t *testing.T) {
	dir := isolatedStateDir(t)
	// PID 0 / port 0 is unusable metadata; the daemon must refuse rather than guess.
	seedLock(t, daemonLockRecord{PID: 0, Port: 0, StateDir: dir})

	deps, _ := newTestDeps(t)
	err := EnforceStartupPolicy(deps, 7890, LaunchOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid daemon lock metadata") {
		t.Fatalf("want invalid-metadata error, got %v", err)
	}
	// The message must name the file so the operator can remove it.
	if !strings.Contains(err.Error(), "daemon.lock.json") {
		t.Fatalf("error = %q, want the lock path", err.Error())
	}
}

func TestEnforceStartupPolicy_Parallel(t *testing.T) {
	t.Run("live foreign daemon blocks a shared state dir", func(t *testing.T) {
		dir := isolatedStateDir(t)
		seedLock(t, daemonLockRecord{PID: 30303, Port: 7920, StateDir: dir})

		deps, _ := newTestDeps(t)
		deps.IsProcessAlive = func(pid int) bool { return pid == 30303 }
		deps.TerminatePID = func(int, bool) { t.Fatal("parallel mode must never kill the incumbent") }
		deps.TryShutdown = func(int) bool { t.Fatal("parallel mode must never shut down the incumbent"); return false }

		err := EnforceStartupPolicy(deps, 7921, LaunchOptions{Parallel: true})
		if err == nil || !strings.Contains(err.Error(), "isolated --state-dir") {
			t.Fatalf("want isolated state-dir guidance, got %v", err)
		}
	})

	t.Run("dead owner's lock is reclaimed", func(t *testing.T) {
		dir := isolatedStateDir(t)
		seedLock(t, daemonLockRecord{PID: 30304, Port: 7920, StateDir: dir})

		deps, _ := newTestDeps(t)
		deps.IsProcessAlive = func(int) bool { return false }
		if err := EnforceStartupPolicy(deps, 7921, LaunchOptions{Parallel: true}); err != nil {
			t.Fatalf("a dead owner should not block parallel mode, got %v", err)
		}
		if rec, _ := readDaemonLockFile(); rec != nil {
			t.Fatalf("stale lock should be gone, got %+v", *rec)
		}
	})

	t.Run("invalid metadata is rejected", func(t *testing.T) {
		dir := isolatedStateDir(t)
		seedLock(t, daemonLockRecord{PID: -1, Port: 0, StateDir: dir})

		deps, _ := newTestDeps(t)
		err := EnforceStartupPolicy(deps, 7921, LaunchOptions{Parallel: true})
		if err == nil || !strings.Contains(err.Error(), "isolated --state-dir") {
			t.Fatalf("want isolated state-dir guidance, got %v", err)
		}
	})
}

func TestEnforceStartupPolicy_PIDMismatch(t *testing.T) {
	t.Run("port still serving -> refuse takeover", func(t *testing.T) {
		dir := isolatedStateDir(t)
		seedLock(t, daemonLockRecord{PID: 51515, Port: 7900, StateDir: dir, Version: "0.7.7"})

		deps, _ := newTestDeps(t)
		deps.IsProcessAlive = func(pid int) bool { return pid == 51515 }
		deps.ReadPIDFile = func(int) int { return 51516 } // mismatch
		deps.IsServerRunning = func(int) bool { return true }
		deps.TerminatePID = func(int, bool) { t.Fatal("must not kill on an unproven ownership mismatch") }

		err := EnforceStartupPolicy(deps, 7901, LaunchOptions{})
		if err == nil || !strings.Contains(err.Error(), "ownership mismatch") {
			t.Fatalf("want ownership mismatch error, got %v", err)
		}
		if rec, _ := readDaemonLockFile(); rec == nil {
			t.Fatal("a refused takeover must leave the incumbent's lock in place")
		}
	})

	t.Run("port idle -> reclaim the stale lock", func(t *testing.T) {
		dir := isolatedStateDir(t)
		seedLock(t, daemonLockRecord{PID: 61616, Port: 7930, StateDir: dir, Version: "0.7.7"})

		deps, log := newTestDeps(t)
		deps.IsProcessAlive = func(pid int) bool { return pid == 61616 }
		deps.ReadPIDFile = func(int) int { return 61617 } // mismatch
		deps.IsServerRunning = func(int) bool { return false }
		deps.TerminatePID = func(int, bool) { t.Fatal("a stale-lock reclaim must not signal anything") }
		removedPIDFile := 0
		deps.RemovePIDFile = func(port int) { removedPIDFile = port }

		if err := EnforceStartupPolicy(deps, 7931, LaunchOptions{}); err != nil {
			t.Fatalf("an idle port should let the stale lock be reclaimed, got %v", err)
		}
		if rec, _ := readDaemonLockFile(); rec != nil {
			t.Fatalf("stale lock should be gone, got %+v", *rec)
		}
		if removedPIDFile != 7930 {
			t.Fatalf("the stale port's pid file should be removed, got port %d", removedPIDFile)
		}
		if log.find("daemon_lock_reclaimed_stale_mismatch") == nil {
			t.Fatal("expected daemon_lock_reclaimed_stale_mismatch lifecycle event")
		}
	})
}

func TestEnforceStartupPolicy_Takeover(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// newDeps builds the common "incumbent is an older version, alive, owns its
	// port" setup that all takeover paths start from.
	newDeps := func(t *testing.T) (Deps, *recordingLogger) {
		d, log := newTestDeps(t)
		d.Version = "0.9.0" // strictly newer => an upgrade takeover
		d.IsProcessAlive = func(pid int) bool { return pid == 42424 }
		d.ReadPIDFile = func(int) int { return 42424 } // ownership proven
		d.IsServerRunning = func(int) bool { return true }
		return d, log
	}

	t.Run("HTTP shutdown alone releases the port", func(t *testing.T) {
		dir := isolatedStateDir(t)
		freezeClock(t, base)
		seedLock(t, daemonLockRecord{PID: 42424, Port: 7890, StateDir: dir, Version: "0.7.7",
			UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)})

		deps, log := newDeps(t)
		shutdownPort := 0
		deps.TryShutdown = func(port int) bool { shutdownPort = port; return true }
		deps.WaitForPortRelease = func(int, time.Duration) bool { return true }
		deps.TerminatePID = func(int, bool) { t.Fatal("a graceful shutdown must not escalate to a signal") }

		if err := EnforceStartupPolicy(deps, 7891, LaunchOptions{}); err != nil {
			t.Fatalf("upgrade takeover should succeed, got %v", err)
		}
		if shutdownPort != 7890 {
			t.Fatalf("want HTTP shutdown of the incumbent port, got %d", shutdownPort)
		}
		if rec, _ := readDaemonLockFile(); rec != nil {
			t.Fatalf("the lock should be cleared after takeover, got %+v", *rec)
		}
		if log.find("daemon_takeover") == nil {
			t.Fatal("expected a daemon_takeover lifecycle event")
		}
	})

	t.Run("escalates SIGTERM then SIGKILL when the port stays held", func(t *testing.T) {
		dir := isolatedStateDir(t)
		freezeClock(t, base)
		seedLock(t, daemonLockRecord{PID: 42424, Port: 7890, StateDir: dir, Version: "0.7.7",
			UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)})

		deps, _ := newDeps(t)
		deps.TryShutdown = func(int) bool { return false }
		waits := 0
		deps.WaitForPortRelease = func(int, time.Duration) bool {
			waits++
			return waits >= 3 // frees only after the SIGKILL
		}
		var forces []bool
		deps.TerminatePID = func(pid int, force bool) {
			if pid != 42424 {
				t.Fatalf("signalled the wrong pid: %d", pid)
			}
			forces = append(forces, force)
		}

		if err := EnforceStartupPolicy(deps, 7891, LaunchOptions{}); err != nil {
			t.Fatalf("takeover should succeed after escalation, got %v", err)
		}
		if len(forces) != 2 || forces[0] || !forces[1] {
			t.Fatalf("want SIGTERM then SIGKILL, got force flags %v", forces)
		}
	})

	t.Run("a port that never frees is a hard error, not a silent pass", func(t *testing.T) {
		dir := isolatedStateDir(t)
		freezeClock(t, base)
		seedLock(t, daemonLockRecord{PID: 42424, Port: 7890, StateDir: dir, Version: "0.7.7",
			UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)})

		deps, _ := newDeps(t)
		deps.TryShutdown = func(int) bool { return false }
		deps.WaitForPortRelease = func(int, time.Duration) bool { return false }

		err := EnforceStartupPolicy(deps, 7891, LaunchOptions{})
		if err == nil || !strings.Contains(err.Error(), "failed to takeover") {
			t.Fatalf("want a takeover failure error, got %v", err)
		}
		// Rule 25: the lock must survive a failed takeover — clearing it would
		// hand the port to a daemon that never actually won it.
		if rec, _ := readDaemonLockFile(); rec == nil {
			t.Fatal("a failed takeover must not clear the incumbent's lock")
		}
	})

	t.Run("a healthy same-version incumbent is deferred to, not killed", func(t *testing.T) {
		dir := isolatedStateDir(t)
		freezeClock(t, base)
		stubInstallEpoch(t, 1000)
		seedLock(t, daemonLockRecord{PID: 42424, Port: 7890, StateDir: dir, Version: "0.9.0",
			InstallEpoch: 1000, UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)})

		deps, _ := newDeps(t)
		deps.FetchHealth = healthyAt("0.9.0")
		deps.TerminatePID = func(int, bool) { t.Fatal("a healthy peer must never be signalled") }
		deps.TryShutdown = func(int) bool { t.Fatal("a healthy peer must never be shut down"); return false }

		if err := EnforceStartupPolicy(deps, 7891, LaunchOptions{}); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("want ErrDeferToHealthyDaemon, got %v", err)
		}
		if rec, _ := readDaemonLockFile(); rec == nil {
			t.Fatal("deferring must leave the incumbent's lock intact")
		}
	})
}

func TestPersistCurrentLockAndRemoveIfOwned(t *testing.T) {
	dir := isolatedStateDir(t)

	if err := PersistCurrentLock(7890, "1.2.3", nil); err != nil {
		t.Fatalf("PersistCurrentLock() error = %v", err)
	}
	rec, err := readDaemonLockFile()
	if err != nil || rec == nil {
		t.Fatalf("lock should exist after persist, got rec=%v err=%v", rec, err)
	}
	if rec.PID != os.Getpid() || rec.Port != 7890 || rec.Version != "1.2.3" || rec.StateDir != dir {
		t.Fatalf("persisted record mismatch: %+v", *rec)
	}
	if rec.UpdatedAt == "" {
		t.Fatal("persisted record must carry an UpdatedAt stamp for the grace window")
	}

	// The on-disk field names are a cross-version contract: an older daemon has to
	// read a newer daemon's lock. Pin them explicitly.
	raw, err := os.ReadFile(daemonLockPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("lock file is not valid JSON: %v", err)
	}
	for _, key := range []string{"pid", "port", "state_dir", "version", "updated_at"} {
		if _, ok := asMap[key]; !ok {
			t.Errorf("lock JSON is missing the %q field", key)
		}
	}

	// A different owner's lock is left alone...
	RemoveLockIfOwned(os.Getpid() + 1)
	if rec, _ := readDaemonLockFile(); rec == nil {
		t.Fatal("RemoveLockIfOwned must not clear a lock owned by someone else")
	}
	// ...and our own is cleared.
	RemoveLockIfOwned(os.Getpid())
	if rec, _ := readDaemonLockFile(); rec != nil {
		t.Fatalf("RemoveLockIfOwned should have cleared our lock, got %+v", *rec)
	}
	// Idempotent: clearing an absent lock is a no-op.
	RemoveLockIfOwned(os.Getpid())
}

func TestRemoveLockIfOwned_NoLockFile(t *testing.T) {
	isolatedStateDir(t)
	RemoveLockIfOwned(os.Getpid()) // must not panic or error
}

// TestResolveInstallEpoch_MemoizedAndStable pins that the epoch is computed once
// and never changes within a process — a drifting epoch would make this daemon
// win or lose the takeover tiebreaker non-deterministically.
func TestResolveInstallEpoch_MemoizedAndStable(t *testing.T) {
	first := resolveInstallEpoch(nil)
	if second := resolveInstallEpoch(nil); second != first {
		t.Fatalf("install epoch must be stable, got %d then %d", first, second)
	}
	if first < 0 {
		t.Fatalf("install epoch must never be negative, got %d", first)
	}
}

// daemonLockPath is the test-side accessor for the lock path.
func daemonLockPath(t *testing.T) string {
	t.Helper()
	p, err := daemonLockFilePath()
	if err != nil {
		t.Fatalf("daemonLockFilePath() error = %v", err)
	}
	return p
}

// healthyAt returns a FetchHealth stub reporting a reachable daemon at ver.
func healthyAt(ver string) func(context.Context, int, time.Duration) (bool, string, bool) {
	return func(context.Context, int, time.Duration) (bool, string, bool) { return true, ver, false }
}

func TestParseVersionParts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  []int
	}{
		{"0.7.5", []int{0, 7, 5}}, {"1.2.3", []int{1, 2, 3}},
		{"v0.7.5", []int{0, 7, 5}}, {"10.20.30", []int{10, 20, 30}},
		{"0.0.0", []int{0, 0, 0}}, {"1.0", []int{1, 0}}, {"5", []int{5}},
	} {
		got := ParseVersionParts(test.input)
		if len(got) != len(test.want) {
			t.Errorf("ParseVersionParts(%q) = %v, want %v", test.input, got, test.want)
			continue
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("ParseVersionParts(%q)[%d] = %d, want %d", test.input, index, got[index], test.want[index])
			}
		}
	}
}

func TestParseVersionPartsMalformed(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "abc", "v", "1.2.abc", "..."} {
		for _, part := range ParseVersionParts(input) {
			if part < 0 {
				t.Errorf("ParseVersionParts(%q) returned negative part: %d", input, part)
			}
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		candidate string
		current   string
		want      bool
	}{
		{"0.7.6", "0.7.5", true}, {"0.8.0", "0.7.5", true}, {"1.0.0", "0.7.5", true},
		{"0.7.5", "0.7.5", false}, {"0.7.4", "0.7.5", false}, {"0.6.9", "0.7.5", false},
		{"v0.7.6", "v0.7.5", true}, {"v0.7.6", "0.7.5", true}, {"0.7.6", "v0.7.5", true},
		{"0.8", "0.7.5", true}, {"0.7.5.1", "0.7.5", true},
		{"", "0.7.5", false}, {"0.7.6", "", false}, {"abc", "0.7.5", false},
	} {
		if got := IsNewerVersion(test.candidate, test.current); got != test.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", test.candidate, test.current, got, test.want)
		}
	}
}

// The startup grace window exists so a daemon still binding its ports is not
// killed by a near-simultaneous launch. That rationale requires a live process.
// When an instance writes its lock and then exits — a crash, or losing a
// takeover race — the fresh lock kept every subsequent launch deferring for the
// whole window, and each one exited announcing "a healthy daemon is already
// serving" while nothing at all listened on the port. Observed in the field:
// repeated restarts all exited with that message, `lsof -iTCP:7890` empty, and
// the extension unable to reach localhost:7890.
func TestEnforceStartupPolicy_GraceWindowDoesNotDeferToADeadOwner(t *testing.T) {
	dir := isolatedStateDir(t)
	now := time.Now().UTC()
	const deadPID = 999002
	seedLock(t, daemonLockRecord{
		PID: deadPID, Port: 7890, StateDir: dir, Version: "0.8.8",
		UpdatedAt: now.Add(-time.Second).Format(time.RFC3339), // inside the 5s grace
	})
	freezeClock(t, now)

	deps, _ := newTestDeps(t)
	deps.IsProcessAlive = func(int) bool { return false }
	deps.ReadPIDFile = func(int) int { return deadPID }

	if err := EnforceStartupPolicy(deps, 7890, LaunchOptions{}); err != nil {
		t.Fatalf("a dead owner inside the grace window must not block startup, got %v", err)
	}
}

// A live owner inside the window must still be deferred to: that is the
// ping-pong protection the window is for, and it must not regress.
func TestEnforceStartupPolicy_GraceWindowStillDefersToALiveOwner(t *testing.T) {
	dir := isolatedStateDir(t)
	now := time.Now().UTC()
	seedLock(t, daemonLockRecord{
		PID: 999003, Port: 7890, StateDir: dir, Version: "0.8.8",
		UpdatedAt: now.Add(-time.Second).Format(time.RFC3339),
	})
	freezeClock(t, now)

	deps, log := newTestDeps(t)
	deps.IsProcessAlive = func(int) bool { return true }
	deps.ReadPIDFile = func(int) int { return 999003 }

	if err := EnforceStartupPolicy(deps, 7890, LaunchOptions{}); !errors.Is(err, ErrDeferToHealthyDaemon) {
		t.Fatalf("a live starting owner must still be deferred to, got %v", err)
	}
	if evt := log.find("daemon_defer_starting"); evt == nil {
		t.Fatal("deferring to a starting daemon must be logged as such, not as a verified-healthy defer")
	}
}

// The lock is per state directory, not per port, so a daemon serving a
// different port can legitimately block this one. When that happens the
// deferral must name the incumbent's port: the old message interpolated the
// REQUESTED port and announced "a healthy daemon is already serving on port
// 7890" while the incumbent was on 19310 and nothing listened on 7890, sending
// the operator to look for a listener that never existed.
func TestDeferralNamesTheIncumbentPortNotTheRequestedOne(t *testing.T) {
	dir := isolatedStateDir(t)
	now := time.Now().UTC()
	seedLock(t, daemonLockRecord{
		PID: 999004, Port: 19310, StateDir: dir, Version: "0.9.0",
		UpdatedAt: now.Add(-time.Hour).Format(time.RFC3339), // well outside the grace window
	})
	freezeClock(t, now)

	deps, _ := newTestDeps(t)
	deps.IsProcessAlive = func(int) bool { return true }
	deps.ReadPIDFile = func(int) int { return 999004 }
	deps.FetchHealth = func(_ context.Context, probedPort int, _ time.Duration) (bool, string, bool) {
		if probedPort != 19310 {
			t.Fatalf("health probe hit port %d, want the incumbent's 19310", probedPort)
		}
		return true, "0.9.0", false
	}

	err := EnforceStartupPolicy(deps, 7890, LaunchOptions{})
	if !errors.Is(err, ErrDeferToHealthyDaemon) {
		t.Fatalf("want a deferral, got %v", err)
	}
	var deferral *Deferral
	if !errors.As(err, &deferral) {
		t.Fatalf("deferral must carry the incumbent's identity, got %T", err)
	}
	if deferral.Port != 19310 || deferral.PID != 999004 {
		t.Fatalf("deferral = %+v, want pid=999004 port=19310", *deferral)
	}
	if !strings.Contains(err.Error(), "19310") {
		t.Fatalf("error %q must name the incumbent's port", err.Error())
	}
}
