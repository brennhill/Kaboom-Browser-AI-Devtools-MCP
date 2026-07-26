// Purpose: Validates test file and directory paths against project boundaries.
// Why: Isolates path security validation from selector healing logic.
package heal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathNotAllowed is the error code for disallowed paths.
const ErrPathNotAllowed = "path_not_allowed"

// ValidateTestFilePath ensures path is within project directory.
func ValidateTestFilePath(path string, projectDir string) error {
	if path == "" {
		return fmt.Errorf("test file path is required")
	}

	if strings.Contains(path, "..") {
		return fmt.Errorf(ErrPathNotAllowed + ": path contains '..'")
	}

	fullPath := ResolveTestPath(path, projectDir)

	cleanPath := filepath.Clean(fullPath)
	cleanProject := filepath.Clean(projectDir)

	if !strings.HasPrefix(cleanPath, cleanProject) {
		return fmt.Errorf(ErrPathNotAllowed + ": path escapes project directory")
	}

	return nil
}

// ResolveTestPath resolves a relative path against the project directory.
func ResolveTestPath(path, projectDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectDir, path)
}

// ValidateBatchDir validates a directory path for batch healing.
func ValidateBatchDir(testDir, projectDir string) (string, error) {
	if err := ValidateTestFilePath(testDir, projectDir); err != nil {
		return "", err
	}

	fullPath := ResolveTestPath(testDir, projectDir)

	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s: directory not found: %s", ErrTestFileNotFound, testDir)
		}
		return "", fmt.Errorf("failed to access directory: %w", err)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf("invalid_param: test_dir must be a directory")
	}

	return fullPath, nil
}
