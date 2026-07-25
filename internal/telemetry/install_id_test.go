// install_id_test.go — Tests for random install ID generation and persistence.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
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

// Regression: defaultKaboomDir() used to derive ~/.kaboom directly, ignoring
// KABOOM_STATE_DIR / XDG_STATE_HOME. It must route through state.RootDir() so a
// project-isolated daemon persists install_id under the configured state root
// (consistent with every other Kaboom artifact) instead of scattering it to
// ~/.kaboom. Fails before the fix, passes after.
func TestDefaultKaboomDir_HonorsStateDirEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(state.StateDirEnv, dir) // t.Setenv forbids t.Parallel and auto-restores
	t.Cleanup(resetKaboomDir)

	resetInstallIDState()
	resetKaboomDir() // recompute kaboomDir now that KABOOM_STATE_DIR is set

	if kaboomDir != dir {
		t.Fatalf("kaboomDir = %q, want %q (KABOOM_STATE_DIR must be honored)", kaboomDir, dir)
	}

	// The install_id file must actually land under the configured root.
	id := GetInstallID()
	data, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("install_id not written under KABOOM_STATE_DIR: %v", err)
	}
	if string(data) != id {
		t.Fatalf("install_id file content = %q, want %q", string(data), id)
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

	// Create a directory where the install_id file would be, making ReadFile fail.
	idPath := filepath.Join(dir, "install_id")
	if err := os.Mkdir(idPath, 0000); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}
	defer os.Chmod(idPath, 0700) // cleanup

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string even on read failure", id)
	}
}
