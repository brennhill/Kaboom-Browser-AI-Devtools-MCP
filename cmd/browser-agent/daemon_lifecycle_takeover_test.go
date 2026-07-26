// Purpose: Tests for health/version/grace-aware takeover decisions — a healthy
// compatible daemon is never killed; a stalled or older one is replaced.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyExistingDaemon(t *testing.T) {
	server, err := NewServer(filepath.Join(t.TempDir(), "classify.log"), 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	oldNow, oldProbe, oldSleep, oldVer, oldEpoch := daemonNow, daemonProbeHealth, daemonSleep, version, daemonInstallEpoch
	defer func() {
		daemonNow = oldNow
		daemonProbeHealth = oldProbe
		daemonSleep = oldSleep
		version = oldVer
		daemonInstallEpoch = oldEpoch
	}()
	daemonNow = func() time.Time { return base }
	daemonSleep = func(time.Duration) {} // never really sleep in tests
	version = "0.8.7"
	// Neutralize the install-epoch tiebreaker for these version/grace/health cases:
	// our epoch equals every lock's epoch, so same-version behavior is decided by the
	// existing rules (not "latest install wins", which has its own test).
	daemonInstallEpoch = func() int64 { return 1000 }

	// Lock written a minute ago => outside the startup grace window. Epoch matches
	// ours (neutral tiebreaker).
	oldLock := func(v string) *daemonLockRecord {
		return &daemonLockRecord{PID: 1, Port: 7890, Version: v, InstallEpoch: 1000, UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)}
	}

	t.Run("healthy same version -> defer (never kill a healthy daemon)", func(t *testing.T) {
		daemonProbeHealth = func(int) (bool, string, bool) { return true, "0.8.7", false }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.7")); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("want defer, got %v", err)
		}
	})

	t.Run("healthy NEWER existing -> defer (never downgrade)", func(t *testing.T) {
		daemonProbeHealth = func(int) (bool, string, bool) { return true, "0.9.0", false }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.9.0")); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("want defer, got %v", err)
		}
	})

	t.Run("older registered version -> upgrade takeover", func(t *testing.T) {
		probed := false
		daemonProbeHealth = func(int) (bool, string, bool) { probed = true; return true, "0.8.6", false }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.6")); err != nil {
			t.Fatalf("upgrade should take over, got %v", err)
		}
		if probed {
			t.Fatalf("a known upgrade should not need a health probe")
		}
	})

	t.Run("stalled (no /health after retries) -> takeover", func(t *testing.T) {
		probeCalls := 0
		daemonProbeHealth = func(int) (bool, string, bool) { probeCalls++; return false, "", false }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("stalled should take over, got %v", err)
		}
		if probeCalls != daemonHealthProbeRetries {
			t.Fatalf("want %d probe retries before declaring stalled, got %d", daemonHealthProbeRetries, probeCalls)
		}
	})

	t.Run("young same-version daemon -> defer via grace, without probing", func(t *testing.T) {
		probed := false
		daemonProbeHealth = func(int) (bool, string, bool) { probed = true; return true, "0.8.7", false }
		young := &daemonLockRecord{PID: 1, Port: 7890, Version: "0.8.7", InstallEpoch: 1000, UpdatedAt: base.Add(-time.Second).Format(time.RFC3339)}
		if err := classifyExistingDaemon(server, 7890, young); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("young lock should defer (grace), got %v", err)
		}
		if probed {
			t.Fatalf("grace should defer without a health probe (avoids racing a still-binding daemon)")
		}
	})

	t.Run("future-dated lock (clock skew) -> defer via grace, without probing", func(t *testing.T) {
		probed := false
		daemonProbeHealth = func(int) (bool, string, bool) { probed = true; return false, "", false }
		// Lock timestamped 2s in the FUTURE (backward clock step): age is negative,
		// which must still be treated as "brand new" and deferred, not taken over.
		future := &daemonLockRecord{PID: 1, Port: 7890, Version: "0.8.7", InstallEpoch: 1000, UpdatedAt: base.Add(2 * time.Second).Format(time.RFC3339)}
		if err := classifyExistingDaemon(server, 7890, future); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("future-dated (negative-age) lock should defer via grace, got %v", err)
		}
		if probed {
			t.Fatalf("a future-dated lock is brand-new; grace should defer without a health probe")
		}
	})

	t.Run("healthy but live version older than us -> upgrade takeover", func(t *testing.T) {
		daemonProbeHealth = func(int) (bool, string, bool) { return true, "0.8.6", false }
		rec := oldLock("") // registered version unknown; rely on live probe
		if err := classifyExistingDaemon(server, 7890, rec); err != nil {
			t.Fatalf("older live version should take over, got %v", err)
		}
	})

	// finding L: a connection-refused probe is definitive (nothing is listening), so
	// takeover must NOT burn the retry budget's sleeps on it. Only an ambiguous
	// timeout warrants retrying.
	t.Run("connection-refused probe takes over WITHOUT retrying (L)", func(t *testing.T) {
		probeCalls := 0
		daemonProbeHealth = func(int) (bool, string, bool) { probeCalls++; return false, "", true } // refused = gone
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("a refused (gone) daemon should take over, got %v", err)
		}
		if probeCalls != 1 {
			t.Fatalf("connection-refused is definitive — want exactly 1 probe, got %d", probeCalls)
		}
	})

	t.Run("timeout probe retries the full budget before takeover (L)", func(t *testing.T) {
		probeCalls := 0
		daemonProbeHealth = func(int) (bool, string, bool) { probeCalls++; return false, "", false } // ambiguous/busy
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("a timed-out daemon should take over after retries, got %v", err)
		}
		if probeCalls != daemonHealthProbeRetries {
			t.Fatalf("an ambiguous timeout should retry the full budget, want %d probes, got %d", daemonHealthProbeRetries, probeCalls)
		}
	})

	t.Run("a healthy DIFFERENT-version answer skips the retry loop (L)", func(t *testing.T) {
		probeCalls := 0
		daemonProbeHealth = func(int) (bool, string, bool) { probeCalls++; return true, "0.8.6", false } // reachable, older
		if err := classifyExistingDaemon(server, 7890, oldLock("")); err != nil {
			t.Fatalf("older live version should take over, got %v", err)
		}
		if probeCalls != 1 {
			t.Fatalf("a reachable answer must break the loop on the first probe, got %d probes", probeCalls)
		}
	})

	server.logs.shutdownAsyncLogger(2 * time.Second)
}
