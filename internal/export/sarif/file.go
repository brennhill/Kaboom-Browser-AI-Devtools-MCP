// Purpose: Resolves output paths and writes SARIF JSON files to disk with symlink-safe resolution.
// Why: Isolates file I/O and path validation from SARIF data conversion.
package sarif

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveExistingPath resolves symlinks on the longest existing prefix of the path.
// For paths where the file doesn't exist yet, it resolves the nearest existing
// ancestor and appends the remaining path components.
func resolveExistingPath(path string) string {
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	// Path doesn't exist; resolve parent and append this component
	parent := filepath.Dir(path)
	if parent == path {
		return path // reached root
	}
	return filepath.Join(resolveExistingPath(parent), filepath.Base(path))
}

// isPathUnderResolvedDir checks if resolvedPath is under an allowed directory.
func isPathUnderResolvedDir(resolvedPath, dir string) bool {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	return strings.HasPrefix(resolvedPath, resolved+string(os.PathSeparator))
}

// validateSARIFSavePath checks that the path is under an allowed directory.
func validateSARIFSavePath(absPath, resolvedPath string) error {
	if isPathUnderResolvedDir(resolvedPath, os.TempDir()) {
		return nil
	}
	if cwd, err := os.Getwd(); err == nil && isPathUnderResolvedDir(resolvedPath, cwd) {
		return nil
	}
	return fmt.Errorf("save_to path must be under the current working directory or temp directory: %s", absPath)
}

// saveSARIFToFile writes the SARIF log to the specified path with security checks.
func saveSARIFToFile(log *SARIFLog, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	resolvedPath := resolveExistingPath(absPath)
	if err := validateSARIFSavePath(absPath, resolvedPath); err != nil {
		return err
	}

	// #nosec G301 -- 0755 for export directory is appropriate
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SARIF: %w", err)
	}

	// #nosec G306 -- export files are intentionally world-readable
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write SARIF file: %w", err)
	}
	return nil
}
