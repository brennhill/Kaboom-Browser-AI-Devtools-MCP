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
	// XDGConfigHomeEnv is consulted by os.UserConfigDir on Linux. Exported so
	// tests can neutralise it: on Linux a set XDG_CONFIG_HOME makes
	// os.UserConfigDir succeed even with HOME empty, so "no home" tests that
	// clear only HOME pass on macOS and fail on Linux CI.
	XDGConfigHomeEnv = "XDG_CONFIG_HOME"
	appName          = "kaboom"
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

// LegacyRootDir returns the historical runtime root used by earlier versions
// (os.UserConfigDir()/kaboom, e.g. ~/Library/Application Support/kaboom).
func LegacyRootDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user config directory: %w", err)
	}
	return filepath.Join(configDir, appName), nil
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

// LegacyDefaultLogFile returns the previous default log file path.
func LegacyDefaultLogFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, "kaboom-logs.jsonl"), nil
}

// CrashLogFile returns the panic crash log file path.
func CrashLogFile() (string, error) {
	return InRoot("logs", "crash.log")
}

// LegacyCrashLogFile returns the previous crash log file path.
func LegacyCrashLogFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, "kaboom-crash.log"), nil
}

// PIDFile returns the PID file path for the given server port.
func PIDFile(port int) (string, error) {
	return InRoot("run", "kaboom-"+strconv.Itoa(port)+".pid")
}

// LegacyPIDFile returns the historical PID file path for the given server port.
func LegacyPIDFile(port int) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".kaboom-"+strconv.Itoa(port)+".pid"), nil
}

// RecordingsDir returns the recordings directory.
func RecordingsDir() (string, error) {
	return InRoot("recordings")
}

// ScreenshotsDir returns the screenshots directory.
//
// LAYOUT INVARIANT: flat — no subdirectories. Both the standard capture flow
// (server_routes_media_screenshots.go writes `<host>-<ts>-<corr>.jpg`) and
// the draw-mode flow (server_routes_media_draw_mode.go writes
// `draw_<ts>_tab<id>_<rand>.png`) write directly into this directory. The
// retention sweep in cmd/browser-agent/screenshot_cleanup.go is non-recursive
// and relies on this; if a future feature wants subdirs, update the sweep
// first.
func ScreenshotsDir() (string, error) {
	return InRoot("screenshots")
}

// ScreenshotImageExts is the set of file extensions written by screenshot
// producers and managed by the retention sweep. Keep extensions lowercase;
// callers that match against this set should normalize via strings.ToLower.
// Adding a producer that writes a new format (WebP, AVIF, HEIC) MUST add the
// matching extension here so the sweep retires it on the same retention
// schedule as JPEG/PNG.
var ScreenshotImageExts = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
}

// LegacyRecordingsDir returns the historical recordings directory.
func LegacyRecordingsDir() (string, error) {
	root, err := LegacyRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "recordings"), nil
}

// SettingsFile returns the extension settings cache file path.
func SettingsFile() (string, error) {
	return InRoot("settings", "extension-settings.json")
}

// LegacySettingsFile returns the historical settings cache file path.
func LegacySettingsFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".kaboom-settings.json"), nil
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
