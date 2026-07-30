// Tests for the install-epoch "latest install always wins" takeover tiebreaker.
// At the SAME version, a strictly newer install epoch supersedes an older one, so
// two same-version installs (e.g. ~/.kaboom/bin vs an npm-global copy) resolve a
// deterministic winner instead of thrashing.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package daemonlife

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// --- computeInstallEpoch ------------------------------------------------------

func TestComputeInstallEpoch_StampFileWins(t *testing.T) {
	exe := func() (string, error) { return "/opt/app/bin/kaboom", nil }
	stat := func(string) (os.FileInfo, error) {
		t.Fatal("stat should not be reached when stamp is valid")
		return nil, nil
	}
	readFile := func(p string) ([]byte, error) {
		if p != filepath.Join("/opt/app/bin", installEpochStampName) {
			t.Fatalf("read wrong path: %s", p)
		}
		return []byte("  1730000000123456789\n"), nil
	}
	if got := computeInstallEpoch(exe, stat, readFile, nil); got != 1730000000123456789 {
		t.Fatalf("want stamp value, got %d", got)
	}
}

func TestComputeInstallEpoch_FallsBackToBinaryMtime(t *testing.T) {
	mtime := time.Date(2026, 7, 26, 1, 2, 3, 456, time.UTC)
	exe := func() (string, error) { return "/x/kaboom", nil }
	stat := func(string) (os.FileInfo, error) { return fakeFileInfo{mod: mtime}, nil }
	readFile := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	if got := computeInstallEpoch(exe, stat, readFile, nil); got != mtime.UnixNano() {
		t.Fatalf("want mtime nanos %d, got %d", mtime.UnixNano(), got)
	}
}

func TestComputeInstallEpoch_InvalidStampFallsBackToMtime(t *testing.T) {
	mtime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exe := func() (string, error) { return "/x/kaboom", nil }
	stat := func(string) (os.FileInfo, error) { return fakeFileInfo{mod: mtime}, nil }
	readFile := func(string) ([]byte, error) { return []byte("not-a-number"), nil }
	diagnostics := statediag.NewCollector()
	if got := computeInstallEpoch(exe, stat, readFile, diagnostics); got != mtime.UnixNano() {
		t.Fatalf("invalid stamp should fall back to mtime; got %d", got)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "install_epoch_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable install-epoch warning", got)
	}
}

func TestComputeInstallEpoch_NoExecutable_ReturnsZero(t *testing.T) {
	exe := func() (string, error) { return "", os.ErrNotExist }
	stat := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	readFile := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	if got := computeInstallEpoch(exe, stat, readFile, nil); got != 0 {
		t.Fatalf("want 0 when executable unknown, got %d", got)
	}
}

// --- sameNonEmptyVersion ------------------------------------------------------

func TestSameNonEmptyVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.8.8", "0.8.8", true},
		{"v0.8.8", "0.8.8", true},
		{"0.8.8", "0.9.0", false},
		{"0.9.0", "0.8.8", false},
		{"", "0.8.8", false},
		{"0.8.8", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := sameNonEmptyVersion(c.a, c.b); got != c.want {
			t.Errorf("sameNonEmptyVersion(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

// --- classifyExistingDaemon epoch tiebreaker ----------------------------------

func TestClassifyExistingDaemon_InstallEpoch(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	freezeClock(t, base)

	deps, _ := newTestDeps(t)
	deps.Version = "0.8.8"

	probe := func(d *Deps, fn func() (bool, string, bool)) {
		d.FetchHealth = func(context.Context, int, time.Duration) (bool, string, bool) { return fn() }
	}

	// A record written a minute ago (outside grace) at the same version, with a
	// given install epoch.
	lock := func(epoch int64) *daemonLockRecord {
		return &daemonLockRecord{
			PID: 1, Port: 7890, Version: "0.8.8", InstallEpoch: epoch,
			UpdatedAt: base.Add(-time.Minute).Format(time.RFC3339),
		}
	}
	// A record written just now (inside the 5s grace window).
	youngLock := func(epoch int64) *daemonLockRecord {
		return &daemonLockRecord{
			PID: 1, Port: 7890, Version: "0.8.8", InstallEpoch: epoch,
			UpdatedAt: base.Add(-time.Second).Format(time.RFC3339),
		}
	}

	t.Run("same version, NEWER install epoch -> take over (latest install wins)", func(t *testing.T) {
		stubInstallEpoch(t, 2000)
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.8.8", false }) // incumbent is healthy
		if err := classifyExistingDaemon(d, 7890, lock(1000)); err != nil {
			t.Fatalf("newer install should take over a healthy same-version incumbent, got %v", err)
		}
	})

	t.Run("same version, OLDER install epoch, healthy -> defer", func(t *testing.T) {
		stubInstallEpoch(t, 1000)
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.8.8", false })
		if err := classifyExistingDaemon(d, 7890, lock(2000)); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("older install must defer to a healthy newer install, got %v", err)
		}
	})

	t.Run("same version, EQUAL install epoch, healthy -> defer (no thrash)", func(t *testing.T) {
		stubInstallEpoch(t, 1000)
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.8.8", false })
		if err := classifyExistingDaemon(d, 7890, lock(1000)); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("equal epoch must defer (never ping-pong same install), got %v", err)
		}
	})

	t.Run("newer install epoch fires even inside the startup grace window", func(t *testing.T) {
		stubInstallEpoch(t, 2000)
		d := deps
		// No health probe needed — epoch takeover uses the registered record.
		probe(&d, func() (bool, string, bool) {
			t.Fatal("should not probe: epoch decides first")
			return false, "", false
		})
		if err := classifyExistingDaemon(d, 7890, youngLock(1000)); err != nil {
			t.Fatalf("newer install should supersede even a just-started older one, got %v", err)
		}
	})

	t.Run("our epoch 0 (unknown) never takes over on epoch alone", func(t *testing.T) {
		stubInstallEpoch(t, 0)
		d := deps
		probe(&d, func() (bool, string, bool) { return true, "0.8.8", false })
		if err := classifyExistingDaemon(d, 7890, lock(0)); !errors.Is(err, ErrDeferToHealthyDaemon) {
			t.Fatalf("unknown epoch must not trigger takeover; got %v", err)
		}
	})
}

// fakeFileInfo is a minimal os.FileInfo for mtime-based tests.
type fakeFileInfo struct{ mod time.Time }

func (f fakeFileInfo) Name() string       { return "kaboom" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (f fakeFileInfo) ModTime() time.Time { return f.mod }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }
