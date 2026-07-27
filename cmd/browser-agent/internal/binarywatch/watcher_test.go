// Purpose: Tests for binary file change detection.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package binarywatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const testVersionCheckTimeout = 15 * time.Second

func TestBinaryWatcherState_UpgradeInfo_Default(t *testing.T) {
	t.Parallel()
	s := &State{}
	pending, ver, at := s.UpgradeInfo()
	if pending || ver != "" || !at.IsZero() {
		t.Fatalf("default state: pending=%v ver=%q at=%v, want false/empty/zero", pending, ver, at)
	}
}

func TestBinaryWatcherState_UpgradeInfo_AfterDetection(t *testing.T) {
	t.Parallel()
	now := time.Now()
	s := &State{}
	s.mu.Lock()
	s.upgradePending = true
	s.detectedVersion = "0.8.0"
	s.detectedAt = now
	s.mu.Unlock()

	pending, ver, at := s.UpgradeInfo()
	if !pending || ver != "0.8.0" || at != now {
		t.Fatalf("after detection: pending=%v ver=%q at=%v", pending, ver, at)
	}
}

func TestBinaryChanged_DetectsModtime(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &State{execPath: tmp}
	// First call always returns false (caches initial state)
	changed, err := s.binaryChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("first call should return false (initial cache)")
	}

	// Overwrite with different size to guarantee change detection.
	if err := os.WriteFile(tmp, []byte("v2-longer"), 0o755); err != nil {
		t.Fatal(err)
	}

	changed, err = s.binaryChanged()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed after modtime+size change")
	}
}

func TestBinaryChanged_NoChangeWhenSame(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &State{execPath: tmp}
	s.binaryChanged() // cache initial

	changed, err := s.binaryChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no change for same file")
	}
}

func TestBinaryChanged_ErrorWhenMissing(t *testing.T) {
	t.Parallel()
	s := &State{execPath: "/nonexistent/binary"}
	_, err := s.binaryChanged()
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}
func TestVerifyBinaryVersion_Timeout(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := verifyBinaryVersionWithTimeout(tmp, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestCheckForUpgrade_DetectsNewer(t *testing.T) {
	t.Parallel()
	// Create a script that reports a newer version
	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\necho 'kaboom v0.8.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &State{execPath: tmp, versionCheckTimeout: testVersionCheckTimeout}
	got := s.checkForUpgrade("0.7.5")
	if !got {
		t.Fatal("checkForUpgrade should detect newer version")
	}
	pending, ver, _ := s.UpgradeInfo()
	if !pending || ver != "0.8.0" {
		t.Fatalf("after checkForUpgrade: pending=%v ver=%q", pending, ver)
	}
}

func TestCheckForUpgrade_IgnoresOlder(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\necho 'kaboom v0.7.4'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &State{execPath: tmp, versionCheckTimeout: testVersionCheckTimeout}
	got := s.checkForUpgrade("0.7.5")
	if got {
		t.Fatal("checkForUpgrade should not detect older version")
	}
}

func TestCheckForUpgrade_IgnoresSame(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\necho 'kaboom v0.7.5'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &State{execPath: tmp, versionCheckTimeout: testVersionCheckTimeout}
	got := s.checkForUpgrade("0.7.5")
	if got {
		t.Fatal("checkForUpgrade should not detect same version")
	}
}

func TestCheckForUpgrade_TimeoutIsolationAcrossParallelStates(t *testing.T) {
	t.Parallel()

	timeouts := make(chan time.Duration, 2)
	nowFn := func() time.Time { return time.Unix(1_700_000_000, 0) }

	makeState := func(timeout time.Duration, reportedVersion string) *State {
		return &State{
			execPath:            "/tmp/ignored",
			versionCheckTimeout: timeout,
			verifyVersion: func(_ string, gotTimeout time.Duration) (string, error) {
				timeouts <- gotTimeout
				return reportedVersion, nil
			},
			now: nowFn,
		}
	}

	fastState := makeState(75*time.Millisecond, "0.8.0")
	slowState := makeState(2*time.Second, "0.9.0")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if !fastState.checkForUpgrade("0.7.0") {
			t.Error("fastState should detect upgrade")
		}
	}()
	go func() {
		defer wg.Done()
		if !slowState.checkForUpgrade("0.7.0") {
			t.Error("slowState should detect upgrade")
		}
	}()
	wg.Wait()

	close(timeouts)
	seen := map[time.Duration]int{}
	for timeout := range timeouts {
		seen[timeout]++
	}

	if seen[75*time.Millisecond] != 1 || seen[2*time.Second] != 1 {
		t.Fatalf("expected isolated per-state timeouts {75ms:1, 2s:1}, got %v", seen)
	}
}

func TestStartBinaryWatcher_ContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	var called bool
	s := Start(ctx, "0.7.5", func(string) { called = true }, func() {})
	if s == nil {
		t.Fatal("Start returned nil")
	}

	cancel()
	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)

	if called {
		t.Fatal("onUpgrade should not be called without binary change")
	}
}

func TestStartBinaryWatcher_DisabledByEnvVar(t *testing.T) {
	t.Setenv("KABOOM_NO_AUTO_UPGRADE", "1")

	s := Start(context.Background(), "0.7.5", func(string) {}, func() {})
	if s != nil {
		t.Fatal("Start should return nil when disabled")
	}
}

func TestStartBinaryWatcher_DetectsUpgrade(t *testing.T) {
	t.Parallel()

	tmp := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\necho 'kaboom v0.7.5'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var upgradeMu sync.Mutex
	var upgradeVersion string
	var shutdownCalled bool

	s := startBinaryWatcherWithConfig(ctx, "0.7.5",
		func(newVer string) {
			upgradeMu.Lock()
			upgradeVersion = newVer
			upgradeMu.Unlock()
		},
		func() {
			upgradeMu.Lock()
			shutdownCalled = true
			upgradeMu.Unlock()
		},
		binaryWatcherConfig{
			resolveExecutablePath: func() (string, error) { return tmp, nil },
			watchInterval:         50 * time.Millisecond,
			upgradeGracePeriod:    50 * time.Millisecond,
			versionCheckTimeout:   testVersionCheckTimeout,
		},
	)
	if s == nil {
		t.Fatal("Start returned nil")
	}

	// Wait until watcher caches baseline file state to avoid a race where the
	// first watcher read observes the upgraded file as its initial snapshot.
	cacheDeadline := time.Now().Add(2 * time.Second)
	cached := false
	for time.Now().Before(cacheDeadline) {
		s.mu.Lock()
		cached = !s.lastModTime.IsZero()
		s.mu.Unlock()
		if cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !cached {
		t.Fatal("watcher did not cache baseline binary state before upgrade write")
	}

	// Now replace binary with newer version
	updatedScript := []byte("#!/bin/sh\n# upgraded build marker\necho 'kaboom v0.8.0'\n")
	if err := os.WriteFile(tmp, updatedScript, 0o755); err != nil {
		t.Fatal(err)
	}
	// Ensure mtime changes even on coarse-grained filesystems.
	bumped := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(tmp, bumped, bumped); err != nil {
		t.Fatalf("failed to bump binary mtime: %v", err)
	}

	// Wait for detection + version verification + grace period with polling
	// instead of fixed sleeps to reduce flake under variable CI load.
	deadline := time.Now().Add(12 * time.Second)
	var gotVer string
	var gotShutdown bool
	for time.Now().Before(deadline) {
		upgradeMu.Lock()
		gotVer = upgradeVersion
		gotShutdown = shutdownCalled
		upgradeMu.Unlock()
		if gotVer == "0.8.0" && gotShutdown {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if gotVer != "0.8.0" {
		t.Fatalf("expected upgrade to 0.8.0, got %q", gotVer)
	}
	if !gotShutdown {
		t.Fatal("expected shutdown to be triggered after grace period")
	}
}

func TestUpgradeMarker_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "last-upgrade.json")

	if err := WriteMarker("0.7.5", "0.8.0", markerPath); err != nil {
		t.Fatalf("WriteMarker() error = %v", err)
	}

	marker, err := ReadAndClearMarker(markerPath)
	if err != nil {
		t.Fatalf("ReadAndClearMarker() error = %v", err)
	}
	if marker == nil {
		t.Fatal("expected non-nil marker")
	}
	if marker.FromVersion != "0.7.5" || marker.ToVersion != "0.8.0" {
		t.Fatalf("marker = %+v, want from=0.7.5 to=0.8.0", marker)
	}

	// File should be removed
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatal("marker file should be removed after read")
	}

	// Second read returns nil
	marker2, err := ReadAndClearMarker(markerPath)
	if err != nil {
		t.Fatalf("second ReadAndClearMarker() error = %v", err)
	}
	if marker2 != nil {
		t.Fatal("expected nil on second read")
	}
}

func TestUpgradeMarker_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "last-upgrade.json")

	if err := os.WriteFile(markerPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	marker, err := ReadAndClearMarker(markerPath)
	if err != nil {
		t.Fatalf("ReadAndClearMarker() should not error on invalid JSON, got %v", err)
	}
	if marker != nil {
		t.Fatal("expected nil for invalid JSON")
	}
	// File should still be cleaned up
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatal("invalid marker file should be removed")
	}
}

func TestUpgradeMarker_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "last-upgrade.json")

	data := Marker{FromVersion: "0.7.0", ToVersion: "0.8.0", Timestamp: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.Marshal(data)
	if err := os.WriteFile(markerPath, b, 0o644); err != nil {
		t.Fatal(err)
	}

	marker, err := ReadAndClearMarker(markerPath)
	if err != nil {
		t.Fatalf("ReadAndClearMarker() error = %v", err)
	}
	if marker == nil || marker.FromVersion != "0.7.0" || marker.ToVersion != "0.8.0" {
		t.Fatalf("marker = %+v", marker)
	}
}

func TestBinaryWatcherState_ThreadSafety(t *testing.T) {
	t.Parallel()

	s := &State{}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.UpgradeInfo()
		}()
	}
	wg.Wait()
}
