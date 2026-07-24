// Purpose: Tests for state save, load, list, and delete operations.
// Docs: docs/features/feature/state-time-travel/index.md

package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootDirUsesOverride(t *testing.T) {
	base := t.TempDir()
	override := filepath.Join(base, "..", filepath.Base(base), "custom-state")

	t.Setenv(StateDirEnv, override)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}

	want, err := filepath.Abs(override)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", override, err)
	}
	want = filepath.Clean(want)

	if got != want {
		t.Fatalf("RootDir() = %q, want %q", got, want)
	}
}

func TestRootDirUsesXDGStateHome(t *testing.T) {
	xdgHome := t.TempDir()

	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, xdgHome)

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}

	want := filepath.Join(xdgHome, appName)
	if got != want {
		t.Fatalf("RootDir() = %q, want %q", got, want)
	}
}

func TestRuntimePathsUnderRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	logFile, err := DefaultLogFile()
	if err != nil {
		t.Fatalf("DefaultLogFile() error = %v", err)
	}
	if want := filepath.Join(root, "logs", "kaboom.jsonl"); logFile != want {
		t.Fatalf("DefaultLogFile() = %q, want %q", logFile, want)
	}

	crashFile, err := CrashLogFile()
	if err != nil {
		t.Fatalf("CrashLogFile() error = %v", err)
	}
	if want := filepath.Join(root, "logs", "exit-diagnostics.log"); crashFile != want {
		t.Fatalf("CrashLogFile() = %q, want %q", crashFile, want)
	}

	pidFile, err := PIDFile(7890)
	if err != nil {
		t.Fatalf("PIDFile() error = %v", err)
	}
	if want := filepath.Join(root, "run", "kaboom-7890.pid"); pidFile != want {
		t.Fatalf("PIDFile() = %q, want %q", pidFile, want)
	}

	settingsFile, err := SettingsFile()
	if err != nil {
		t.Fatalf("SettingsFile() error = %v", err)
	}
	if want := filepath.Join(root, "settings", "extension-settings.json"); settingsFile != want {
		t.Fatalf("SettingsFile() = %q, want %q", settingsFile, want)
	}
}

func TestLegacyPathsUseUserConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() error = %v", err)
	}

	legacyRoot, err := LegacyRootDir()
	if err != nil {
		t.Fatalf("LegacyRootDir() error = %v", err)
	}
	if want := filepath.Join(configDir, appName); legacyRoot != want {
		t.Fatalf("LegacyRootDir() = %q, want %q", legacyRoot, want)
	}

	legacyLog, err := LegacyDefaultLogFile()
	if err != nil {
		t.Fatalf("LegacyDefaultLogFile() error = %v", err)
	}
	if want := filepath.Join(home, "kaboom-logs.jsonl"); legacyLog != want {
		t.Fatalf("LegacyDefaultLogFile() = %q, want %q", legacyLog, want)
	}
}
