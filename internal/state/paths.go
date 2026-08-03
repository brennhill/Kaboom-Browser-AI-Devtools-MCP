// Purpose: Resolves runtime state, logs, pid, and recording filesystem paths for Kaboom.
// Why: Ensures all runtime artifacts use a consistent, configurable directory policy.
// Docs: docs/features/feature/project-isolation/index.md

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// StateDirEnv overrides the default runtime state root.
	StateDirEnv = "KABOOM_STATE_DIR"

	xdgStateHomeEnv = "XDG_STATE_HOME"
	appName         = "kaboom"
)

// RootDir returns the runtime state root for Kaboom.
// Resolution order:
//  1. KABOOM_STATE_DIR (if set)
//  2. XDG_STATE_HOME/kaboom (if XDG_STATE_HOME is set)
//  3. ~/.kaboom (cross-platform dotfolder)
func RootDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(StateDirEnv)); override != "" {
		return normalizePath(override)
	}

	if xdg := strings.TrimSpace(os.Getenv(xdgStateHomeEnv)); xdg != "" {
		root, err := normalizePath(xdg)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, appName), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".kaboom"), nil
}

// ProjectDir returns the centralized project-scoped persistence directory
// under ~/.kaboom/projects/{abs-path}. The leading path separator is stripped
// so the absolute project path becomes a relative subpath.
func ProjectDir(projectPath string) (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve project path: %w", err)
	}
	rel := strings.TrimPrefix(filepath.Clean(absPath), string(os.PathSeparator))
	return filepath.Join(root, "projects", rel), nil
}

// DefaultLogFile returns the default structured log file path.
func DefaultLogFile() (string, error) {
	return InRoot("logs", "kaboom.jsonl")
}

// CrashLogFile returns the exit-diagnostics log file path. Despite the function
// name, this log records EVERY process exit — normal
// stdin-EOF bridge exits and clean shutdowns as well as panics — so the file is
// named exit-diagnostics.log, not crash.log: a large file here is churn, not crashes.
func CrashLogFile() (string, error) {
	return InRoot("logs", "exit-diagnostics.log")
}

// PIDFile returns the PID file path for the given server port.
func PIDFile(port int) (string, error) {
	return InRoot("run", "kaboom-"+strconv.Itoa(port)+".pid")
}

// RecordingsDir returns the recordings directory.
func RecordingsDir() (string, error) {
	return InRoot("recordings")
}

// ScreenshotsDir returns the screenshots directory.
func ScreenshotsDir() (string, error) {
	return InRoot("screenshots")
}

// EvidenceDir returns the local content-addressed QA evidence directory.
func EvidenceDir() (string, error) {
	return InRoot("evidence")
}

// SettingsFile returns the extension settings cache file path.
func SettingsFile() (string, error) {
	return InRoot("settings", "extension-settings.json")
}

// SecurityConfigFile returns the security configuration path.
func SecurityConfigFile() (string, error) {
	return InRoot("security", "security.json")
}

// UpgradeMarkerFile returns the path for the binary upgrade marker file.
func UpgradeMarkerFile() (string, error) {
	return InRoot("run", "last-upgrade.json")
}

// InRoot returns a path rooted under RootDir with additional path elements.
func InRoot(parts ...string) (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	all := make([]string, 0, len(parts)+1)
	all = append(all, root)
	all = append(all, parts...)
	return filepath.Join(all...), nil
}

func normalizePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("resolve_path: path argument is empty. Provide a non-empty file path")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %q: %w", path, err)
	}
	return filepath.Clean(absPath), nil
}
