// instancegov_test.go — Proves the machine-wide admission sequence: the singleton
// lock elects one production daemon, a newer build is handed the lock rather than
// racing for it, and test daemons evict the oldest instead of multiplying.

package instancegov_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/proclock"
)

func baseConfig(t *testing.T) instancegov.Config {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(instancereg.DirEnv, dir)
	return instancegov.Config{
		Role:     instancereg.RoleDaemon,
		Ports:    []int{7890, 7891},
		Version:  "0.9.0",
		StateDir: filepath.Join(dir, "state"),
		LockPath: filepath.Join(dir, "daemon.singleton.lock"),
		Policy:   instancegov.Policy{DaemonCap: 1, ParallelCap: 2, BridgeCap: 4},
		Now:      time.Now,
	}
}

func TestFirstProductionDaemonProceedsAndRegisters(t *testing.T) {
	cfg := baseConfig(t)
	result, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = result.Release() })

	if result.Outcome != instancegov.OutcomeProceed {
		t.Fatalf("Admit() = %v, want Proceed", result.Outcome)
	}
	if result.Lock == nil {
		t.Fatal("Admit() proceeded without holding the singleton lock")
	}
	records, _ := instancereg.List()
	if len(records) != 1 || records[0].Role != instancereg.RoleDaemon {
		t.Fatalf("registry = %+v, want one daemon record", records)
	}
}

// The headline guarantee, enforced by the kernel rather than by a timestamp
// heuristic: a second production daemon cannot bind, whatever its state dir.
func TestSecondProductionDaemonDefersToTheLockHolder(t *testing.T) {
	cfg := baseConfig(t)
	first, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("first Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second := cfg
	second.StateDir = filepath.Join(t.TempDir(), "other-state-dir")
	result, err := instancegov.Admit(second)
	if err != nil {
		t.Fatalf("second Admit() error = %v", err)
	}
	if result.Outcome != instancegov.OutcomeDefer {
		t.Fatalf("second Admit() = %v, want Defer", result.Outcome)
	}
	if result.Lock != nil {
		t.Fatal("a deferring instance acquired the singleton lock")
	}
	records, _ := instancereg.List()
	if len(records) != 1 {
		t.Fatalf("registry has %d records, want 1 (the deferring instance must not register)", len(records))
	}
}

// A genuine upgrade must still be able to take over, but by ASKING the incumbent
// to stand down and then acquiring the freed lock -- never by racing for it.
func TestNewerVersionRequestsHandoffAndProceeds(t *testing.T) {
	cfg := baseConfig(t)
	incumbent, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("incumbent Admit() error = %v", err)
	}

	var askedPID int
	upgrade := cfg
	upgrade.Version = "0.10.0"
	upgrade.RequestShutdown = func(rec instancereg.Record) error {
		askedPID = rec.PID
		// The incumbent stands down, releasing the kernel lock.
		return incumbent.Release()
	}

	result, err := instancegov.Admit(upgrade)
	if err != nil {
		t.Fatalf("upgrade Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = result.Release() })

	if result.Outcome != instancegov.OutcomeProceed {
		t.Fatalf("upgrade Admit() = %v, want Proceed", result.Outcome)
	}
	if askedPID == 0 {
		t.Error("upgrade did not ask the incumbent to stand down")
	}
}

// An older build must never displace a newer one.
func TestOlderVersionDefersToNewerIncumbent(t *testing.T) {
	cfg := baseConfig(t)
	cfg.Version = "0.10.0"
	incumbent, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("incumbent Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = incumbent.Release() })

	older := cfg
	older.Version = "0.9.0"
	older.RequestShutdown = func(instancereg.Record) error {
		t.Error("an older build asked a newer incumbent to stand down")
		return nil
	}
	result, err := instancegov.Admit(older)
	if err != nil {
		t.Fatalf("older Admit() error = %v", err)
	}
	if result.Outcome != instancegov.OutcomeDefer {
		t.Fatalf("older Admit() = %v, want Defer", result.Outcome)
	}
}

// Test daemons are capped, and going over the cap evicts the oldest rather than
// refusing to start -- a test run must always be able to proceed.
func TestParallelDaemonOverCapEvictsOldest(t *testing.T) {
	cfg := baseConfig(t)
	cfg.Parallel = true
	cfg.Policy.ParallelCap = 1

	first, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("first parallel Admit() error = %v", err)
	}
	if first.Outcome != instancegov.OutcomeProceed {
		t.Fatalf("first parallel Admit() = %v, want Proceed", first.Outcome)
	}

	var terminated []int
	second := cfg
	second.Terminate = func(pid int, force bool) error {
		terminated = append(terminated, pid)
		return nil
	}
	result, err := instancegov.Admit(second)
	if err != nil {
		t.Fatalf("second parallel Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = result.Release() })

	if result.Outcome != instancegov.OutcomeProceed {
		t.Fatalf("second parallel Admit() = %v, want Proceed after eviction", result.Outcome)
	}
	if len(terminated) != 1 {
		t.Fatalf("terminated %v, want exactly one eviction", terminated)
	}
	if len(result.Evicted) != 1 {
		t.Fatalf("Admit() reported %d evictions, want 1", len(result.Evicted))
	}
}

// Parallel daemons must not contend for the production singleton lock: they are
// isolated by design and capped by count, not by mutual exclusion.
func TestParallelDaemonsDoNotTakeTheSingletonLock(t *testing.T) {
	cfg := baseConfig(t)
	cfg.Parallel = true
	result, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = result.Release() })
	if result.Lock != nil {
		t.Fatal("a parallel daemon took the production singleton lock")
	}

	// The production lock must therefore still be free.
	lock, err := proclock.Acquire(cfg.LockPath)
	if err != nil {
		t.Fatalf("production lock was held by a parallel daemon: %v", err)
	}
	_ = lock.Release()
}

func TestReleaseDeregistersAndUnlocks(t *testing.T) {
	cfg := baseConfig(t)
	result, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if err := result.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	records, _ := instancereg.List()
	if len(records) != 0 {
		t.Fatalf("Release() left %d registry records", len(records))
	}
	lock, err := proclock.Acquire(cfg.LockPath)
	if err != nil {
		t.Fatalf("Release() did not free the singleton lock: %v", err)
	}
	_ = lock.Release()
	if err := result.Release(); err != nil {
		t.Fatalf("second Release() error = %v, want idempotent", err)
	}
}

// If the incumbent refuses to stand down, the upgrade must report that rather
// than proceeding without the lock and binding a second daemon anyway.
func TestFailedHandoffDoesNotProceed(t *testing.T) {
	cfg := baseConfig(t)
	incumbent, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("incumbent Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = incumbent.Release() })

	upgrade := cfg
	upgrade.Version = "0.10.0"
	upgrade.HandoffTimeout = 50 * time.Millisecond
	upgrade.RequestShutdown = func(instancereg.Record) error {
		return errors.New("incumbent refused")
	}
	result, err := instancegov.Admit(upgrade)
	if err == nil && result.Outcome == instancegov.OutcomeProceed {
		t.Fatal("Admit() proceeded without the singleton lock after a failed handoff")
	}
	if result.Lock != nil {
		t.Fatal("Admit() reported a lock it does not hold")
	}
}

// A deferring daemon must be able to NAME the incumbent. The incumbent publishes
// its identity into the lock file just after acquiring it, so a daemon that loses
// the race by microseconds can read an empty payload; without a retry it reports
// only "a daemon is already serving", which sends an operator looking for a
// process they cannot identify.
func TestDeferralNamesTheIncumbentEvenWhenItJustAcquired(t *testing.T) {
	cfg := baseConfig(t)
	first, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("first Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second := cfg
	result, err := instancegov.Admit(second)
	if err != nil {
		t.Fatalf("second Admit() error = %v", err)
	}
	if result.Outcome != instancegov.OutcomeDefer {
		t.Fatalf("second Admit() = %v, want Defer", result.Outcome)
	}
	if result.DeferTo == nil {
		t.Fatal("Admit() deferred without naming the incumbent")
	}
	if result.DeferTo.PID == 0 {
		t.Errorf("incumbent record has no pid: %+v", result.DeferTo)
	}
	if len(result.DeferTo.Ports) == 0 {
		t.Errorf("incumbent record names no ports: %+v", result.DeferTo)
	}
}

// The genuine cross-process race: the winner has the kernel lock but has not yet
// published who it is. A loser that reads at that instant must wait briefly rather
// than reporting an anonymous incumbent.
func TestDeferralWaitsBrieflyForTheIncumbentToPublish(t *testing.T) {
	cfg := baseConfig(t)

	// Take the lock the way a winner does, but publish nothing yet.
	held, err := proclock.Acquire(cfg.LockPath)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	t.Cleanup(func() { _ = held.Release() })

	// The winner publishes shortly after acquiring.
	go func() {
		time.Sleep(120 * time.Millisecond)
		payload, _ := json.Marshal(instancereg.Record{
			PID: 4242, Role: instancereg.RoleDaemon, Ports: []int{7890, 7891}, Version: "0.9.0",
		})
		_ = held.Write(payload)
	}()

	result, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if result.Outcome != instancegov.OutcomeDefer {
		t.Fatalf("Admit() = %v, want Defer", result.Outcome)
	}
	if result.DeferTo == nil || result.DeferTo.PID != 4242 {
		t.Fatalf("Admit() deferred to %+v, want the incumbent pid 4242", result.DeferTo)
	}
}

// At the SAME version, a strictly newer INSTALL supersedes an older one. Without
// this, two same-version installs (an npm-global copy and ~/.kaboom/bin) have no
// way to pick a winner, and a fresh install can never displace the daemon the
// previous install left running. Migrated from daemonlife's takeover policy when
// the kernel lock became the single admission authority.
func TestNewerInstallSupersedesAtTheSameVersion(t *testing.T) {
	cfg := baseConfig(t)
	cfg.InstallEpoch = 1000
	incumbent, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("incumbent Admit() error = %v", err)
	}

	var asked bool
	fresh := cfg
	fresh.InstallEpoch = 2000 // same version, newer install
	fresh.RequestShutdown = func(instancereg.Record) error {
		asked = true
		return incumbent.Release()
	}
	result, err := instancegov.Admit(fresh)
	if err != nil {
		t.Fatalf("fresh install Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = result.Release() })

	if !asked {
		t.Error("a newer install did not ask the older one to stand down")
	}
	if result.Outcome != instancegov.OutcomeProceed {
		t.Fatalf("fresh install Admit() = %v, want Proceed", result.Outcome)
	}
}

// The tiebreaker must not ping-pong: an equal or older install always defers, so
// two same-version daemons can never take turns evicting each other.
func TestEqualOrOlderInstallDefersAtTheSameVersion(t *testing.T) {
	for name, epoch := range map[string]int64{"equal": 1000, "older": 500} {
		t.Run(name, func(t *testing.T) {
			cfg := baseConfig(t)
			cfg.InstallEpoch = 1000
			incumbent, err := instancegov.Admit(cfg)
			if err != nil {
				t.Fatalf("incumbent Admit() error = %v", err)
			}
			t.Cleanup(func() { _ = incumbent.Release() })

			other := cfg
			other.InstallEpoch = epoch
			other.RequestShutdown = func(instancereg.Record) error {
				t.Errorf("install epoch %d asked a %s install to stand down", epoch, name)
				return nil
			}
			result, err := instancegov.Admit(other)
			if err != nil {
				t.Fatalf("Admit() error = %v", err)
			}
			if result.Outcome != instancegov.OutcomeDefer {
				t.Fatalf("Admit() = %v, want Defer", result.Outcome)
			}
		})
	}
}

// A missing epoch on either side must never trigger a takeover: an unknown install
// age is not evidence of being newer.
func TestUnknownInstallEpochNeverSupersedes(t *testing.T) {
	cfg := baseConfig(t)
	cfg.InstallEpoch = 0
	incumbent, err := instancegov.Admit(cfg)
	if err != nil {
		t.Fatalf("incumbent Admit() error = %v", err)
	}
	t.Cleanup(func() { _ = incumbent.Release() })

	other := cfg
	other.InstallEpoch = 0
	other.RequestShutdown = func(instancereg.Record) error {
		t.Error("an unknown install epoch triggered a takeover")
		return nil
	}
	if result, _ := instancegov.Admit(other); result.Outcome != instancegov.OutcomeDefer {
		t.Fatalf("Admit() = %v, want Defer", result.Outcome)
	}
}
