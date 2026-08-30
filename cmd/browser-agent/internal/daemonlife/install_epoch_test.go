// Tests for the install-epoch "latest install always wins" takeover tiebreaker.
// At the SAME version, a strictly newer install epoch supersedes an older one, so
// two same-version installs (e.g. ~/.kaboom/bin vs an npm-global copy) resolve a
// deterministic winner instead of thrashing.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package daemonlife

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
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

func TestComputeInstallEpochCanonicalFaultsUseDeterministicMtimeFallback(t *testing.T) {
	mtime := time.Date(2026, 8, 3, 1, 2, 3, 0, time.UTC)
	for _, kind := range []statefault.Kind{statefault.Read, statefault.Corruption, statefault.PartialWrite} {
		t.Run(string(kind), func(t *testing.T) {
			scenario := statefault.New(kind, "private-install-epoch")
			diagnostics := statediag.NewCollector()
			got := computeInstallEpoch(
				func() (string, error) { return "/x/kaboom", nil },
				func(string) (os.FileInfo, error) { return fakeFileInfo{mod: mtime}, nil },
				func(string) ([]byte, error) {
					if kind == statefault.Read {
						return nil, scenario.Error()
					}
					return scenario.Payload([]byte("1730000000123456789")), nil
				},
				diagnostics,
			)
			if got != mtime.UnixNano() {
				t.Fatalf("epoch = %d, want mtime fallback %d", got, mtime.UnixNano())
			}
			if incidents := diagnostics.Snapshot(); len(incidents) != 1 || incidents[0].Name != "install_epoch_state" {
				t.Fatalf("diagnostics = %#v, want install epoch incident", incidents)
			}
		})
	}
}

// fakeFileInfo is a minimal os.FileInfo for mtime-based tests.
type fakeFileInfo struct{ mod time.Time }

func (f fakeFileInfo) Name() string       { return "kaboom" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (f fakeFileInfo) ModTime() time.Time { return f.mod }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

// The install epoch is the admission tiebreaker at equal versions, so it must be
// stable for a process's whole life: a value that changed between two reads could
// make a daemon supersede itself. Moved here from lifecycle_policy_test.go when
// the PID-lock startup policy was deleted.
func TestResolveInstallEpoch_MemoizedAndStable(t *testing.T) {
	first := resolveInstallEpoch(nil)
	if second := resolveInstallEpoch(nil); second != first {
		t.Fatalf("install epoch must be stable, got %d then %d", first, second)
	}
	if first < 0 {
		t.Fatalf("install epoch must never be negative, got %d", first)
	}
}
