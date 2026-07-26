// Purpose: Enforces file path security: upload-dir scoping, symlink resolution, sensitive path denylist, and case-fold matching.
// Docs: docs/features/feature/file-upload/index.md

// Package uploadsec is the safety kernel for the upload stages: everything that
// decides whether an input may be acted on at all.
//
// It covers three kinds of check that share one property — they are pure policy
// with no knowledge of upload wire types or stage orchestration:
//
//   - Path safety (Security, ValidateUploadDir, ValidateFilePath, denylist, hardlink).
//   - Network safety (SSRF: DNS-pinned dialing, private-range rejection, safe transport).
//   - Input safety (HTTP method/URL/cookie validation and the AppleScript, SendKeys
//     and Content-Disposition sanitizers that keep untrusted text from breaking
//     out of the syntax it is embedded in).
//
// uploadsec is a leaf: it imports nothing else from internal/upload.
package uploadsec

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================
// Upload Security Configuration
// ============================================

// Security holds the validated upload security configuration.
// Created at startup, immutable after initialization.
type Security struct {
	// uploadDir is the resolved absolute path to the allowed upload directory.
	// Empty string means no upload-dir was specified (Stage 1 still works with denylist).
	uploadDir string

	// userDenyPatterns are additional patterns from --upload-deny-pattern flags.
	userDenyPatterns []string
}

// NewSecurity creates a Security directly without startup validation (test use only).
func NewSecurity(uploadDir string, userDenyPatterns []string) *Security {
	return &Security{uploadDir: uploadDir, userDenyPatterns: userDenyPatterns}
}

// ============================================
// Startup Validation
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
// Returns a configured Security or an error that should halt startup.
func ValidateUploadDir(rawDir string, userDenyPatterns []string) (*Security, error) {
	sec := &Security{userDenyPatterns: userDenyPatterns}

	if rawDir == "" {
		return sec, nil
	}

	resolved, err := resolveAndValidateDir(rawDir)
	if err != nil {
		return nil, err
	}

	sec.uploadDir = resolved
	return sec, nil
}

// resolveAndValidateDir performs all validation steps on the upload directory path.
func resolveAndValidateDir(rawDir string) (string, error) {
	if !filepath.IsAbs(rawDir) {
		return "", fmt.Errorf("--upload-dir must be an absolute path, got: %s", rawDir)
	}

	info, err := os.Stat(rawDir)
	if err != nil {
		return "", fmt.Errorf("--upload-dir does not exist: %s: %w", rawDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--upload-dir is not a directory: %s", rawDir)
	}

	resolved, err := filepath.EvalSymlinks(rawDir)
	if err != nil {
		return "", fmt.Errorf("--upload-dir: failed to resolve symlinks: %w", err)
	}
	// Reject only when the upload directory ITSELF is a symlink — not when an ancestor
	// component is. Comparing the fully-resolved path against the raw path falsely
	// rejected legitimate directories whose ancestors are OS-level symlinks (macOS
	// /var -> /private/var, Windows 8.3 short paths under %TEMP%). EvalSymlinks above
	// still canonicalizes the path returned for downstream use.
	linfo, lerr := os.Lstat(rawDir)
	if lerr != nil {
		return "", fmt.Errorf("--upload-dir: failed to stat: %w", lerr)
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("--upload-dir must not be a symlink: %s resolves to %s", rawDir, resolved)
	}

	if err := rejectRootOrHome(resolved); err != nil {
		return "", fmt.Errorf("--upload-dir %s: %w", rawDir, err)
	}

	if pattern, matched := MatchesDenylist(resolved); matched {
		return "", fmt.Errorf("--upload-dir matches sensitive path pattern %q: %s", pattern, rawDir)
	}

	return resolved, nil
}

// rejectRootOrHome rejects paths that are filesystem roots or user home directories.
func rejectRootOrHome(resolved string) error {
	// Reject filesystem roots.
	roots := []string{"/", "/home", "/Users", "/root"}
	if runtime.GOOS == "windows" {
		for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			roots = append(roots, fmt.Sprintf("%c:\\", drive))
			roots = append(roots, fmt.Sprintf("%c:\\Users", drive))
		}
	}
	for _, root := range roots {
		if strings.EqualFold(resolved, filepath.Clean(root)) {
			return fmt.Errorf("must be a subdirectory, not a root or system directory")
		}
	}

	// Reject user home directory itself.
	home, err := os.UserHomeDir()
	if err == nil {
		homeResolved, err := filepath.EvalSymlinks(home)
		if err == nil && resolved == homeResolved {
			return fmt.Errorf("must be a subdirectory of your home directory, not the home directory itself")
		}
	}

	return nil
}

// ============================================
// Per-Request File Path Validation
// ============================================

// PathValidationResult contains the validated, resolved file path.
type PathValidationResult struct {
	// ResolvedPath is the cleaned, symlink-resolved absolute path safe to open.
	ResolvedPath string
}

// ValidateFilePath runs the full validation chain on a file path.
// requireUploadDir controls whether the path must be within --upload-dir.
// Stage 1 passes requireUploadDir=false; Stages 2-4 pass requireUploadDir=true.
func (s *Security) ValidateFilePath(rawPath string, requireUploadDir bool) (*PathValidationResult, error) {
	resolved, err := resolveCleanPath(rawPath)
	if err != nil {
		return nil, err
	}

	if err := s.checkDenylists(rawPath, resolved); err != nil {
		return nil, err
	}

	if err := s.checkUploadDirConstraint(rawPath, resolved, requireUploadDir); err != nil {
		return nil, err
	}

	return &PathValidationResult{ResolvedPath: resolved}, nil
}

// resolveCleanPath cleans the path, verifies it is absolute, and resolves symlinks.
func resolveCleanPath(rawPath string) (string, error) {
	cleaned := filepath.Clean(rawPath)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("file_path must be an absolute path. Relative paths are not allowed for security")
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s: %w", rawPath, err)
		}
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return resolved, nil
}

// checkDenylists checks the resolved path against built-in and user deny patterns.
func (s *Security) checkDenylists(rawPath, resolved string) error {
	if pattern, matched := MatchesDenylist(resolved); matched {
		return &PathDeniedError{Path: rawPath, Pattern: pattern, UploadDir: s.uploadDir}
	}
	if pattern, matched := MatchesUserDenylist(resolved, s.userDenyPatterns); matched {
		return &PathDeniedError{Path: rawPath, Pattern: pattern, UploadDir: s.uploadDir}
	}
	return nil
}

// checkUploadDirConstraint verifies the file is within --upload-dir when required.
func (s *Security) checkUploadDirConstraint(rawPath, resolved string, required bool) error {
	if !required {
		return nil
	}
	if s.uploadDir == "" {
		return &UploadDirRequiredError{}
	}
	if !IsWithinDir(resolved, s.uploadDir) {
		return &PathDeniedError{Path: rawPath, Pattern: "outside --upload-dir", UploadDir: s.uploadDir}
	}
	return nil
}

// IsWithinDir checks if filePath is within or equal to dirPath.
// Uses case-insensitive comparison on macOS/Windows.
func IsWithinDir(filePath, dirPath string) bool {
	dirWithSep := dirPath
	if !strings.HasSuffix(dirWithSep, string(filepath.Separator)) {
		dirWithSep += string(filepath.Separator)
	}
	return PathHasPrefixFold(filePath, dirWithSep) || PathsEqualFold(filePath, dirPath)
}

// checkHardlink is defined in security_unix.go and security_windows.go

// ============================================
// Error Types
// ============================================

// PathDeniedError is returned when a file path is blocked by the denylist or upload-dir constraint.
type PathDeniedError struct {
	Path      string
	Pattern   string
	UploadDir string
}

func (e *PathDeniedError) Error() string {
	if e.Pattern == "outside --upload-dir" {
		return fmt.Sprintf("File path %q is outside the allowed upload directory (%s). Move the file there and retry.", e.Path, e.UploadDir)
	}
	msg := fmt.Sprintf("File path %q is not allowed: matches sensitive path pattern %q.", e.Path, e.Pattern)
	if e.UploadDir != "" {
		msg += fmt.Sprintf(" Move the file to your upload directory (%s) and retry.", e.UploadDir)
	}
	return msg
}

// UploadDirRequiredError is returned when Stages 2-4 are attempted without --upload-dir.
type UploadDirRequiredError struct{}

func (e *UploadDirRequiredError) Error() string {
	return "This upload stage requires --upload-dir to be set. Restart the server with --upload-dir=/path/to/folder and move your files there."
}
