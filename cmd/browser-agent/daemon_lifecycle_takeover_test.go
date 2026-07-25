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
	oldNow, oldProbe, oldSleep, oldVer := daemonNow, daemonProbeHealth, daemonSleep, version
	defer func() {
		daemonNow = oldNow
		daemonProbeHealth = oldProbe
		daemonSleep = oldSleep
		version = oldVer
	}()
	daemonNow = func() time.Time { return base }
	daemonSleep = func(time.Duration) {} // never really sleep in tests
	version = "0.8.7"

	// Lock written a minute ago => outside the startup grace window.
	oldLock := func(v string) *daemonLockRecord {
		return &daemonLockRecord{PID: 1, Port: 7890, Version: v, UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339)}
	}

	t.Run("healthy same version -> defer (never kill a healthy daemon)", func(t *testing.T) {
		daemonProbeHealth = func(int) (bool, string) { return true, "0.8.7" }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.7")); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("want defer, got %v", err)
		}
	})

	t.Run("healthy NEWER existing -> defer (never downgrade)", func(t *testing.T) {
		daemonProbeHealth = func(int) (bool, string) { return true, "0.9.0" }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.9.0")); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("want defer, got %v", err)
		}
	})

	t.Run("older registered version -> upgrade takeover", func(t *testing.T) {
		probed := false
		daemonProbeHealth = func(int) (bool, string) { probed = true; return true, "0.8.6" }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.6")); err != nil {
			t.Fatalf("upgrade should take over, got %v", err)
		}
		if probed {
			t.Fatalf("a known upgrade should not need a health probe")
		}
	})

	t.Run("stalled (no /health after retries) -> takeover", func(t *testing.T) {
		probeCalls := 0
		daemonProbeHealth = func(int) (bool, string) { probeCalls++; return false, "" }
		if err := classifyExistingDaemon(server, 7890, oldLock("0.8.7")); err != nil {
			t.Fatalf("stalled should take over, got %v", err)
		}
		if probeCalls != daemonHealthProbeRetries {
			t.Fatalf("want %d probe retries before declaring stalled, got %d", daemonHealthProbeRetries, probeCalls)
		}
	})

	t.Run("young same-version daemon -> defer via grace, without probing", func(t *testing.T) {
		probed := false
		daemonProbeHealth = func(int) (bool, string) { probed = true; return true, "0.8.7" }
		young := &daemonLockRecord{PID: 1, Port: 7890, Version: "0.8.7", UpdatedAt: base.Add(-time.Second).Format(time.RFC3339)}
		if err := classifyExistingDaemon(server, 7890, young); !errors.Is(err, errDeferToHealthyDaemon) {
			t.Fatalf("young lock should defer (grace), got %v", err)
		}
		if probed {
			t.Fatalf("grace should defer without a health probe (avoids racing a still-binding daemon)")
		}
	})

	t.Run("healthy but live version older than us -> upgrade takeover", func(t *testing.T) {
		daemonProbeHealth = func(int) (bool, string) { return true, "0.8.6" }
		rec := oldLock("") // registered version unknown; rely on live probe
		if err := classifyExistingDaemon(server, 7890, rec); err != nil {
			t.Fatalf("older live version should take over, got %v", err)
		}
	})

	server.logs.shutdownAsyncLogger(2 * time.Second)
}
