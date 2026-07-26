// Purpose: Tests for health/version/grace-aware takeover decisions — a healthy
// compatible daemon is never killed; a stalled or older one is replaced.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package daemonlife

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifyExistingDaemon(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	freezeClock(t, base)
	// Neutralize the install-epoch tiebreaker for these version/grace/health cases:
	// our epoch equals every lock's epoch, so same-version behavior is decided by the
	// existing rules (not "latest install wins", which has its own test).
	stubInstallEpoch(t, 1000)

	deps, _ := newTestDeps(t)
	deps.Version = "0.8.7"
	// Default: the incumbent PID is alive. classifyExistingDaemon only consults this
	// seam on the connection-refused path (a refused probe while the PID is alive is
	// not proof of death, so it is retried). Refused-path subtests override it.
	deps.IsProcessAlive = func(int) bool { return true }

	// probe installs a FetchHealth stub returning the given verdict.
	probe := func(d *Deps, fn func() (bool, string, bool)) {
		d.FetchHealth = func(context.Context, int, time.Duration) (bool, string, bool) { return fn() }
	}

	// Lock written a minute ago => outside the startup grace window. Epoch matches
	// ours (neutral tiebreaker).
	oldLock := func(v string) *daemonLockRecord {
		return &daemonLockRecord{PID: 1, Port: 7890, Version: v, InstallEpoch: 1000, UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)}
	}

	t.Run("healthy same version -> defer (never kill a healthy daemon)", func(t *testing.T) {
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.8.7", false })
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.7")); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("want defer, got %v", err)
		}
	})

	t.Run("healthy NEWER existing -> defer (never downgrade)", func(t *testing.T) {
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.9.0", false })
		if err := classifyExistingDaemon(d, 7890, oldLock("0.9.0")); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("want defer, got %v", err)
		}
	})

	t.Run("older registered version -> upgrade takeover", func(t *testing.T) {
		d := deps
		probed := false
		probe(&d, func() (bool, string, bool) { probed = true; return true, "0.8.6", false })
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.6")); err != nil {
			t.Fatalf("upgrade should take over, got %v", err)
		}
		if probed {
			t.Fatalf("a known upgrade should not need a health probe")
		}
	})

	t.Run("stalled (no /health after retries) -> takeover", func(t *testing.T) {
		d := deps
		probeCalls := 0
		probe(&d, func() (bool, string, bool) { probeCalls++; return false, "", false })
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("stalled should take over, got %v", err)
		}
		if probeCalls != daemonHealthProbeRetries {
			t.Fatalf("want %d probe retries before declaring stalled, got %d", daemonHealthProbeRetries, probeCalls)
		}
	})

	t.Run("young same-version daemon -> defer via grace, without probing", func(t *testing.T) {
		d := deps
		probed := false
		probe(&d, func() (bool, string, bool) { probed = true; return true, "0.8.7", false })
		young := &daemonLockRecord{PID: 1, Port: 7890, Version: "0.8.7", InstallEpoch: 1000, UpdatedAt: base.Add(-time.Second).Format(time.RFC3339)}
		if err := classifyExistingDaemon(d, 7890, young); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("young lock should defer (grace), got %v", err)
		}
		if probed {
			t.Fatalf("grace should defer without a health probe (avoids racing a still-binding daemon)")
		}
	})

	t.Run("future-dated lock (clock skew) -> defer via grace, without probing", func(t *testing.T) {
		d := deps
		probed := false
		probe(&d, func() (bool, string, bool) { probed = true; return false, "", false })
		// Lock timestamped 2s in the FUTURE (backward clock step): age is negative,
		// which must still be treated as "brand new" and deferred, not taken over.
		future := &daemonLockRecord{PID: 1, Port: 7890, Version: "0.8.7", InstallEpoch: 1000, UpdatedAt: base.Add(2 * time.Second).Format(time.RFC3339)}
		if err := classifyExistingDaemon(d, 7890, future); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("future-dated (negative-age) lock should defer via grace, got %v", err)
		}
		if probed {
			t.Fatalf("a future-dated lock is brand-new; grace should defer without a health probe")
		}
	})

	t.Run("healthy but live version older than us -> upgrade takeover", func(t *testing.T) {
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.8.6", false })
		rec := oldLock("") // registered version unknown; rely on live probe
		if err := classifyExistingDaemon(d, 7890, rec); err != nil {
			t.Fatalf("older live version should take over, got %v", err)
		}
	})

	// finding L (refined): a connection-refused probe is only DEFINITIVE ("gone")
	// when the incumbent PID is also dead — then takeover must NOT burn the retry
	// budget's sleeps. But a refused probe while the PID is still ALIVE is not proof
	// of death (a healthy daemon can transiently refuse: listen backlog full, or a
	// graceful-shutdown drain), so it is retried to avoid falsely SIGTERM'ing a live
	// peer and feeding the takeover war.
	t.Run("refused + DEAD pid takes over WITHOUT retrying (L)", func(t *testing.T) {
		d := deps
		d.IsProcessAlive = func(int) bool { return false } // genuinely gone
		probeCalls := 0
		probe(&d, func() (bool, string, bool) { probeCalls++; return false, "", true }) // refused
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("a refused daemon whose PID is dead should take over, got %v", err)
		}
		if probeCalls != 1 {
			t.Fatalf("refused + dead PID is definitive — want exactly 1 probe, got %d", probeCalls)
		}
	})

	// Regression for the false-takeover risk in finding L: a healthy daemon that
	// transiently refuses ONE probe (PID alive) must be retried, see health on the
	// retry, and be DEFERRED to — never taken over on a single stray refusal.
	t.Run("refused-then-healthy + ALIVE pid -> defer (no false takeover)", func(t *testing.T) {
		d := deps
		d.IsProcessAlive = func(int) bool { return true } // incumbent is alive
		call := 0
		probe(&d, func() (bool, string, bool) {
			call++
			if call == 1 {
				return false, "", true // transient connection-refused
			}
			return true, "0.8.7", false // healthy on retry
		})
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.7")); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("a transiently-refusing but healthy live daemon must be deferred to, got %v", err)
		}
		if call < 2 {
			t.Fatalf("want a retry after the first refused probe (PID alive), got %d probes", call)
		}
	})

	// A live daemon whose listener stays wedged (refused across the whole window) is
	// still reclaimed after the retry budget — the alive-PID retry must not protect a
	// genuinely stalled daemon forever.
	t.Run("refused-persistently + ALIVE pid -> takeover after full retry budget (L)", func(t *testing.T) {
		d := deps
		d.IsProcessAlive = func(int) bool { return true }
		probeCalls := 0
		probe(&d, func() (bool, string, bool) { probeCalls++; return false, "", true }) // always refused
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("a persistently-wedged live daemon should take over after retries, got %v", err)
		}
		if probeCalls != daemonHealthProbeRetries {
			t.Fatalf("refused + alive PID must retry the full budget, want %d probes, got %d", daemonHealthProbeRetries, probeCalls)
		}
	})

	t.Run("timeout probe retries the full budget before takeover (L)", func(t *testing.T) {
		d := deps
		probeCalls := 0
		probe(&d, func() (bool, string, bool) { probeCalls++; return false, "", false }) // ambiguous/busy
		if err := classifyExistingDaemon(d, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("a timed-out daemon should take over after retries, got %v", err)
		}
		if probeCalls != daemonHealthProbeRetries {
			t.Fatalf("an ambiguous timeout should retry the full budget, want %d probes, got %d", daemonHealthProbeRetries, probeCalls)
		}
	})

	t.Run("a healthy DIFFERENT-version answer skips the retry loop (L)", func(t *testing.T) {
		d := deps
		probeCalls := 0
		probe(&d, func() (bool, string, bool) { probeCalls++; return true, "0.8.6", false }) // reachable, older
		if err := classifyExistingDaemon(d, 7890, oldLock("")); err != nil {
			t.Fatalf("older live version should take over, got %v", err)
		}
		if probeCalls != 1 {
			t.Fatalf("a reachable answer must break the loop on the first probe, got %d probes", probeCalls)
		}
	})
}
