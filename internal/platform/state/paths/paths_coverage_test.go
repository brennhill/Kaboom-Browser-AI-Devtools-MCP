// Purpose: Coverage-expansion tests for state management edge cases and branch paths.
// Docs: docs/features/feature/state-time-travel/index.md

// paths_coverage_test.go — Additional tests targeting uncovered branches in paths.go.
// Covers InRoot error propagation, ProjectDir error paths, Legacy* function
// success and error paths, RootDir XDG normalization errors, and normalizePath
// edge cases.
package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// RootDir — KABOOM_STATE_DIR with spaces/dots (normalization)
// ---------------------------------------------------------------------------

func TestRootDir_OverrideWithTrailingSlash(t *testing.T) {
	tmp := t.TempDir()
	override := tmp + string(os.PathSeparator)
	t.Setenv(StateDirEnv, override)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	// filepath.Clean removes trailing separator
	want := filepath.Clean(tmp)
	if got != want {
		t.Fatalf("RootDir() = %q, want %q", got, want)
	}
}

func TestRootDir_OverrideWhitespaceOnly(t *testing.T) {
	// Whitespace-only override should fall through to XDG or home
	t.Setenv(StateDirEnv, "   ")
	home := t.TempDir()
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	want := filepath.Join(home, ".kaboom")
	if got != want {
		t.Fatalf("RootDir() = %q, want %q (whitespace override should fall through)", got, want)
	}
}

func TestRootDir_XDGWhitespaceOnly(t *testing.T) {
	// Whitespace-only XDG_STATE_HOME should fall through to home
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "   ")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	want := filepath.Join(home, ".kaboom")
	if got != want {
		t.Fatalf("RootDir() = %q, want %q (whitespace XDG should fall through)", got, want)
	}
}

func TestRootDir_OverrideRelativePath(t *testing.T) {
	// Relative path in KABOOM_STATE_DIR should be resolved to absolute
	t.Setenv(StateDirEnv, "relative-state-dir")
	t.Setenv(xdgStateHomeEnv, "")

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("RootDir() = %q, want absolute path", got)
	}
}

func TestRootDir_XDGRelativePath(t *testing.T) {
	// Relative path in XDG_STATE_HOME should be resolved to absolute
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "relative-xdg-dir")

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("RootDir() = %q, want absolute path", got)
	}
	if !strings.HasSuffix(got, filepath.Join("relative-xdg-dir", appName)) {
		t.Fatalf("RootDir() = %q, want suffix %q", got, filepath.Join("relative-xdg-dir", appName))
	}
}

func TestRootDir_XDGAbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, tmp)

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	want := filepath.Join(tmp, appName)
	if got != want {
		t.Fatalf("RootDir() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// InRoot — basic and error propagation
// ---------------------------------------------------------------------------

func TestInRoot_MultipleSegments(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := InRoot("a", "b", "c")
	if err != nil {
		t.Fatalf("InRoot() error = %v", err)
	}
	want := filepath.Join(root, "a", "b", "c")
	if got != want {
		t.Fatalf("InRoot(a,b,c) = %q, want %q", got, want)
	}
}

func TestInRoot_NoParts(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := InRoot()
	if err != nil {
		t.Fatalf("InRoot() error = %v", err)
	}
	if got != root {
		t.Fatalf("InRoot() = %q, want %q", got, root)
	}
}

// ---------------------------------------------------------------------------
// ProjectDir — additional coverage
// ---------------------------------------------------------------------------

func TestProjectDir_RelativePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := ProjectDir("myproject")
	if err != nil {
		t.Fatalf("ProjectDir() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("ProjectDir() = %q, want absolute path", got)
	}
	if !strings.HasPrefix(got, filepath.Join(root, "projects")) {
		t.Fatalf("ProjectDir() = %q, want prefix %q", got, filepath.Join(root, "projects"))
	}
}

func TestProjectDir_WithDotDot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := ProjectDir("/Users/brenn/../brenn/dev")
	if err != nil {
		t.Fatalf("ProjectDir() error = %v", err)
	}
	// The ".." should be cleaned away
	want := filepath.Join(root, "projects", "Users", "brenn", "dev")
	if got != want {
		t.Fatalf("ProjectDir() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// PIDFile — various ports
// ---------------------------------------------------------------------------

func TestPIDFile_Ports(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	tests := []struct {
		port int
		file string
	}{
		{0, "kaboom-0.pid"},
		{80, "kaboom-80.pid"},
		{8080, "kaboom-8080.pid"},
		{65535, "kaboom-65535.pid"},
	}
	for _, tt := range tests {
		got, err := PIDFile(tt.port)
		if err != nil {
			t.Fatalf("PIDFile(%d) error = %v", tt.port, err)
		}
		want := filepath.Join(root, "run", tt.file)
		if got != want {
			t.Fatalf("PIDFile(%d) = %q, want %q", tt.port, got, want)
		}
	}
}
func TestRecordingsDirPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := RecordingsDir()
	if err != nil {
		t.Fatalf("RecordingsDir() error = %v", err)
	}
	if got != filepath.Join(root, "recordings") {
		t.Fatalf("RecordingsDir() = %q, want %q", got, filepath.Join(root, "recordings"))
	}
}

func TestScreenshotsDirPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := ScreenshotsDir()
	if err != nil {
		t.Fatalf("ScreenshotsDir() error = %v", err)
	}
	if got != filepath.Join(root, "screenshots") {
		t.Fatalf("ScreenshotsDir() = %q, want %q", got, filepath.Join(root, "screenshots"))
	}
}

func TestSecurityConfigFilePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := SecurityConfigFile()
	if err != nil {
		t.Fatalf("SecurityConfigFile() error = %v", err)
	}
	want := filepath.Join(root, "security", "security.json")
	if got != want {
		t.Fatalf("SecurityConfigFile() = %q, want %q", got, want)
	}
}

func TestSettingsFilePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := SettingsFile()
	if err != nil {
		t.Fatalf("SettingsFile() error = %v", err)
	}
	want := filepath.Join(root, "settings", "extension-settings.json")
	if got != want {
		t.Fatalf("SettingsFile() = %q, want %q", got, want)
	}
}

func TestDefaultLogFilePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := DefaultLogFile()
	if err != nil {
		t.Fatalf("DefaultLogFile() error = %v", err)
	}
	want := filepath.Join(root, "logs", "kaboom.jsonl")
	if got != want {
		t.Fatalf("DefaultLogFile() = %q, want %q", got, want)
	}
}

func TestCrashLogFilePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := CrashLogFile()
	if err != nil {
		t.Fatalf("CrashLogFile() error = %v", err)
	}
	want := filepath.Join(root, "logs", "exit-diagnostics.log")
	if got != want {
		t.Fatalf("CrashLogFile() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// normalizePath — additional edge cases
// ---------------------------------------------------------------------------

func TestNormalizePath_AbsoluteWithDots(t *testing.T) {
	t.Parallel()

	input := filepath.Join(string(os.PathSeparator), "a", "b", "..", "c", ".", "d")
	got, err := normalizePath(input)
	if err != nil {
		t.Fatalf("normalizePath() error = %v", err)
	}
	want := filepath.Join(string(os.PathSeparator), "a", "c", "d")
	if got != want {
		t.Fatalf("normalizePath(%q) = %q, want %q", input, got, want)
	}
}

func TestNormalizePath_RelativeSimple(t *testing.T) {
	t.Parallel()

	got, err := normalizePath("foo")
	if err != nil {
		t.Fatalf("normalizePath(foo) error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("normalizePath(foo) = %q, want absolute", got)
	}
	if !strings.HasSuffix(got, "foo") {
		t.Fatalf("normalizePath(foo) = %q, want suffix 'foo'", got)
	}
}

func TestNormalizePath_EmptyReturnsError(t *testing.T) {
	t.Parallel()

	_, err := normalizePath("")
	if err == nil {
		t.Fatal("normalizePath(\"\") should return error")
	}
	if !strings.Contains(err.Error(), "resolve_path: path argument is empty") {
		t.Fatalf("normalizePath(\"\") error = %q, want 'resolve_path: path argument is empty'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// StateDirEnv constant
// ---------------------------------------------------------------------------

func TestStateDirEnvConstant(t *testing.T) {
	t.Parallel()

	if StateDirEnv != "KABOOM_STATE_DIR" {
		t.Fatalf("StateDirEnv = %q, want KABOOM_STATE_DIR", StateDirEnv)
	}
}

// ---------------------------------------------------------------------------
// Error paths — HOME unset triggers os.UserHomeDir/os.UserConfigDir failures.
// These tests cannot use t.Parallel() because they use t.Setenv.
// ---------------------------------------------------------------------------

func TestRootDir_ErrorWhenHomeUndefined(t *testing.T) {
	// No KABOOM_STATE_DIR, no XDG, and no HOME => RootDir falls to
	// os.UserHomeDir() which errors with "$HOME is not defined".
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := RootDir()
	if err == nil {
		t.Fatal("RootDir() expected error when HOME is empty, got nil")
	}
	if !strings.Contains(err.Error(), "home directory") {
		t.Fatalf("RootDir() error = %q, want 'home directory'", err.Error())
	}
}

func TestInRoot_ErrorWhenRootDirFails(t *testing.T) {
	// Force RootDir to fail: no override, no XDG, no HOME
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := InRoot("logs")
	if err == nil {
		t.Fatal("InRoot() expected error when RootDir fails, got nil")
	}
}

func TestProjectDir_ErrorWhenRootDirFails(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := ProjectDir("/some/path")
	if err == nil {
		t.Fatal("ProjectDir() expected error when RootDir fails, got nil")
	}
}
func TestDefaultLogFile_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := DefaultLogFile()
	if err == nil {
		t.Fatal("DefaultLogFile() expected error when RootDir fails, got nil")
	}
}

func TestCrashLogFile_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := CrashLogFile()
	if err == nil {
		t.Fatal("CrashLogFile() expected error when RootDir fails, got nil")
	}
}

func TestPIDFile_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := PIDFile(8080)
	if err == nil {
		t.Fatal("PIDFile() expected error when RootDir fails, got nil")
	}
}

func TestRecordingsDir_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := RecordingsDir()
	if err == nil {
		t.Fatal("RecordingsDir() expected error when RootDir fails, got nil")
	}
}

func TestScreenshotsDir_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := ScreenshotsDir()
	if err == nil {
		t.Fatal("ScreenshotsDir() expected error when RootDir fails, got nil")
	}
}

func TestSettingsFile_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := SettingsFile()
	if err == nil {
		t.Fatal("SettingsFile() expected error when RootDir fails, got nil")
	}
}

func TestSecurityConfigFile_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := SecurityConfigFile()
	if err == nil {
		t.Fatal("SecurityConfigFile() expected error when RootDir fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// UpgradeMarkerFile
// ---------------------------------------------------------------------------

func TestUpgradeMarkerFilePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := UpgradeMarkerFile()
	if err != nil {
		t.Fatalf("UpgradeMarkerFile() error = %v", err)
	}
	want := filepath.Join(root, "run", "last-upgrade.json")
	if got != want {
		t.Fatalf("UpgradeMarkerFile() = %q, want %q", got, want)
	}
}

func TestUpgradeMarkerFile_ErrorPropagation(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := UpgradeMarkerFile()
	if err == nil {
		t.Fatal("UpgradeMarkerFile() expected error when RootDir fails, got nil")
	}
}
