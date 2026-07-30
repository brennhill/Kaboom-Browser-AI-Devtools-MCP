// install_id_test.go — Tests for random install ID generation and persistence.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

func TestGetInstallID_GeneratesOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}
}

func TestGetInstallID_PersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id1 := GetInstallID()
	id2 := GetInstallID()
	if id1 != id2 {
		t.Fatalf("GetInstallID() returned different values: %q vs %q", id1, id2)
	}
}

func TestGetInstallID_ReadsFromFile(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Pre-write a known ID file.
	knownID := "aabbccddeeff"
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte(knownID), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != knownID {
		t.Fatalf("GetInstallID() = %q, want %q (from file)", id, knownID)
	}
}

func TestGetInstallID_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", ".strum")
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}

	// Verify directory was created.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("strum dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("strum dir path is not a directory")
	}

	// Verify file was written.
	data, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("install_id file not written: %v", err)
	}
	if string(data) != id {
		t.Fatalf("file content = %q, want %q", string(data), id)
	}
}

// #7: Install ID file with trailing newline should be trimmed.
func TestGetInstallID_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Write ID with trailing newline (common from echo "id" > file).
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("aabbccddeeff\n"), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != "aabbccddeeff" {
		t.Fatalf("GetInstallID() = %q, want %q (should trim whitespace)", id, "aabbccddeeff")
	}
}

// #7: Install ID with spaces and carriage return should be trimmed.
func TestGetInstallID_TrimsCarriageReturn(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("  aabbccddeeff\r\n"), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != "aabbccddeeff" {
		t.Fatalf("GetInstallID() = %q, want %q", id, "aabbccddeeff")
	}
}

// Installation identity is machine-install scoped, not runtime-state scoped.
// UAT and project-isolated daemons routinely override these state roots; letting
// those overrides rotate iid inflates active-install analytics.
func TestDefaultKaboomDir_IgnoresRuntimeStateOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KABOOM_STATE_DIR", filepath.Join(t.TempDir(), "isolated"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "xdg"))
	t.Cleanup(resetKaboomDir)

	resetInstallIDState()
	resetKaboomDir()

	want := filepath.Join(home, ".kaboom")
	if kaboomDir != want {
		t.Fatalf("kaboomDir = %q, want installation root %q", kaboomDir, want)
	}

	id := GetInstallID()
	data, err := os.ReadFile(filepath.Join(want, "install_id"))
	if err != nil {
		t.Fatalf("install_id not written under installation root: %v", err)
	}
	if string(data) != id {
		t.Fatalf("install_id file content = %q, want %q", string(data), id)
	}
}

// Concurrent daemon starts must converge on the one ID that wins file
// creation. A normal overwriting write lets every process cache a different ID.
func TestLoadOrGenerateInstallID_ConcurrentCreatorsConverge(t *testing.T) {
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)

	const creators = 32
	start := make(chan struct{})
	results := make(chan string, creators)
	var wg sync.WaitGroup
	wg.Add(creators)
	for range creators {
		go func() {
			defer wg.Done()
			<-start
			results <- loadOrGenerateInstallID()
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var first string
	for id := range results {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("concurrent creators returned %q and %q", first, id)
		}
	}
}

func TestGetInstallID_ReadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not effective on Windows")
	}

	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	diagnostics := statediag.NewCollector()
	stateRecovery = diagnostics

	// Create a directory where the install_id file would be, making ReadFile fail.
	idPath := filepath.Join(dir, "install_id")
	if err := os.Mkdir(idPath, 0000); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}
	defer os.Chmod(idPath, 0700) // cleanup

	id := GetInstallID()
	if id != "" {
		t.Fatalf("GetInstallID() = %q, want telemetry-suppressing empty ID on read failure", id)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "install_identity_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable install identity warning", got)
	}
}

func TestGetInstallIDReplacesMalformedIdentityOnce(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	diagnostics := statediag.NewCollector()
	stateRecovery = diagnostics

	idPath := filepath.Join(dir, "install_id")
	if err := os.WriteFile(idPath, []byte("not-a-stable-id"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := GetInstallID()
	if !hexPattern.MatchString(first) {
		t.Fatalf("replacement ID = %q, want stable hex ID", first)
	}
	resetInstallIDState()
	stateRecovery = diagnostics
	second := GetInstallID()
	if second != first {
		t.Fatalf("replacement rotated across restart: first=%q second=%q", first, second)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "install_identity_state" {
		t.Fatalf("diagnostics = %#v, want one identity recovery", got)
	}
}
