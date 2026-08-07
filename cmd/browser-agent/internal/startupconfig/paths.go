// paths.go — Resolves startup state, log, and upload paths without process exits.

package startupconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// BuildUploadSecurity resolves and validates the local upload boundary.
func BuildUploadSecurity(enabled bool, directory string, denyPatterns []string) (*uploadsec.Security, error) {
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			if !enabled {
				return &uploadsec.Security{}, nil
			}
			return nil, fmt.Errorf("determine home directory: %w", err)
		}
		directory = filepath.Join(home, "kaboom-upload-dir")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			if !enabled {
				return &uploadsec.Security{}, nil
			}
			return nil, fmt.Errorf("create default upload directory %s: %w", directory, err)
		}
	}
	security, err := uploadsec.ValidateUploadDir(directory, denyPatterns)
	if err != nil {
		return nil, fmt.Errorf("validate upload directory: %w", err)
	}
	return security, nil
}

// NormalizeStateDir resolves a configured state directory and publishes the
// canonical path for all state owners in the process.
func NormalizeStateDir(directory string) (string, error) {
	if directory == "" {
		return "", nil
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve state directory: %w", err)
	}
	resolved := filepath.Clean(absolute)
	if err := os.Setenv(state.StateDirEnv, resolved); err != nil {
		return "", fmt.Errorf("set %s: %w", state.StateDirEnv, err)
	}
	return resolved, nil
}

// ResolveLogFile returns an explicit path unchanged or resolves the canonical
// state log path. State failures use a local temporary fallback and warning.
func ResolveLogFile(logFile string) (string, string) {
	if logFile != "" {
		return logFile, ""
	}
	defaultLogFile, err := state.DefaultLogFile()
	if err == nil {
		return defaultLogFile, ""
	}
	fallback := filepath.Join(os.TempDir(), "kaboom", "logs", "kaboom.jsonl")
	return fallback, fmt.Sprintf("state_dir_unwritable: %v; falling back to %s", err, fallback)
}
