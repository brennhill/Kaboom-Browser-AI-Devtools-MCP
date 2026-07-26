// pidfile.go — PID file and liveness helpers for daemon lifecycle management.
// Why: Centralizes process identity operations used by startup/stop flows and lifecycle tests.
// Docs: docs/features/architecture/index.md

package procctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// PIDFilePath returns the path to the PID file for a given port.
func PIDFilePath(port int) string {
	path, err := state.PIDFile(port)
	if err != nil {
		return ""
	}
	return path
}

// LegacyPIDFilePath returns the old PID path used in previous releases.
func LegacyPIDFilePath(port int) string {
	path, err := state.LegacyPIDFile(port)
	if err != nil {
		return ""
	}
	return path
}

// WritePIDFile writes the current process ID to the PID file.
func WritePIDFile(port int) error {
	path := PIDFilePath(port)
	if path == "" {
		return fmt.Errorf("cannot determine PID file path")
	}
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("cannot create PID directory: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
}

// ReadPIDFile reads the PID from the PID file, returns 0 if not found or invalid.
func ReadPIDFile(port int) int {
	paths := []string{PIDFilePath(port), LegacyPIDFilePath(port)}
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil {
			return pid
		}
	}
	return 0
}

// RemovePIDFile removes the PID file for a given port.
func RemovePIDFile(port int) {
	paths := []string{PIDFilePath(port), LegacyPIDFilePath(port)}
	for _, path := range paths {
		if path != "" {
			_ = os.Remove(path)
		}
	}
}

// IsProcessAlive checks if a process with the given PID is still running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Use Signal(0) to check if process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
