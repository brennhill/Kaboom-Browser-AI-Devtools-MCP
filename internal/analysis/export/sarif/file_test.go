// export_sarif_file_test.go — Tests SARIF file paths and filesystem safety.
// Docs: docs/features/feature/sarif-export/index.md

package sarif

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// File Save Tests
// ============================================

func TestExportSARIF_SaveToFile(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alt text",
			"help": "Add alt attribute",
			"helpUrl": "https://example.com",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<img>", "target": ["img"], "impact": "critical"}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "results.sarif")

	opts := SARIFExportOptions{SaveTo: savePath}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(savePath) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Saved file is empty")
	}

	// Verify it's valid JSON
	var parsed SARIFLog
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Saved file is not valid SARIF JSON: %v", err)
	}

	// The returned log should still be valid
	if log.Version != "2.1.0" {
		t.Errorf("Expected version '2.1.0', got %q", log.Version)
	}
}

func TestExportSARIF_InvalidPath(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{SaveTo: "/nonexistent/path/results.sarif"}
	_, err := ExportSARIF(a11yResult, opts)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestExportSARIF_PathTraversal(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	// Path traversal attempts should be rejected
	badPaths := []string{
		"../../etc/passwd",
		"/etc/passwd",
	}

	for _, path := range badPaths {
		opts := SARIFExportOptions{SaveTo: path}
		_, err := ExportSARIF(a11yResult, opts)
		// Either error or the path should be rejected
		// We allow /tmp and cwd, reject everything else
		if err == nil && !strings.HasPrefix(path, "/tmp") {
			// Check if it's under cwd
			cwd, _ := os.Getwd()
			absPath, _ := filepath.Abs(path)
			if !strings.HasPrefix(absPath, cwd) {
				t.Errorf("Expected error for path traversal %q, got nil", path)
			}
		}
	}
}

// ============================================
// Coverage Gap Tests
// ============================================

func TestSaveSARIFToFile_ValidAbsPath(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alternate text",
			"help": "Images must have alternate text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/image-alt",
			"tags": ["wcag2a", "wcag111"],
			"nodes": [{
				"html": "<img src=\"photo.jpg\">",
				"target": ["img"],
				"impact": "critical"
			}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "subdir", "results.sarif.json")

	opts := SARIFExportOptions{
		SaveTo: savePath,
	}

	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF with save_to failed: %v", err)
	}
	if log == nil {
		t.Fatal("Expected non-nil SARIF log")
	}

	// Verify the file was written
	data, err := os.ReadFile(savePath) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("Failed to read saved SARIF file: %v", err)
	}

	var savedLog SARIFLog
	if err := json.Unmarshal(data, &savedLog); err != nil {
		t.Fatalf("Saved file is not valid SARIF JSON: %v", err)
	}
	if savedLog.Version != "2.1.0" {
		t.Errorf("Expected version 2.1.0, got %q", savedLog.Version)
	}
	if len(savedLog.Runs[0].Results) != 1 {
		t.Errorf("Expected 1 result in saved file, got %d", len(savedLog.Runs[0].Results))
	}
}

func TestSaveSARIFToFile_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	// Use a path under /tmp but with an invalid parent that can't be created
	// (a file pretending to be a directory)
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("I am a file"), 0600); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}
	// Try to create a file inside the blocking file (which is not a directory)
	savePath := filepath.Join(blockingFile, "subdir", "result.sarif.json")

	opts := SARIFExportOptions{
		SaveTo: savePath,
	}

	_, err := ExportSARIF(a11yResult, opts)
	if err == nil {
		t.Fatal("Expected error when MkdirAll fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("Expected 'failed to create directory' error, got: %v", err)
	}
}

func TestExportSARIF_SaveToTempDir(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "label",
			"impact": "critical",
			"description": "Form elements must have labels",
			"help": "Add a label",
			"helpUrl": "https://example.com",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<input>", "target": ["input"], "impact": "critical"}]
		}],
		"passes": [],
		"incomplete": []
	}`)

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "subdir", "output.sarif.json")

	log, err := ExportSARIF(a11yResult, SARIFExportOptions{SaveTo: outPath})
	if err != nil {
		t.Fatalf("ExportSARIF with SaveTo failed: %v", err)
	}
	if log == nil {
		t.Fatal("Expected non-nil log")
	}

	// Verify the file was written
	data, err := os.ReadFile(outPath) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("Failed to read saved SARIF file: %v", err)
	}
	if !strings.Contains(string(data), "label") {
		t.Error("Expected saved file to contain rule ID 'label'")
	}
}

// ============================================
// Coverage: saveSARIFToFile with unwritable directory (line 305)
// ============================================

func TestSaveSARIFToFile_UnwritableDir(t *testing.T) {
	t.Parallel()
	log := &SARIFLog{
		Schema:  "https://example.com/schema",
		Version: "2.1.0",
		Runs:    []sarifRun{},
	}

	// Create a temp dir, then create a subdir that's not writable
	tmpDir := t.TempDir()
	readonlyDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(readonlyDir, 0o555)
	defer os.Chmod(readonlyDir, 0o755)

	outPath := filepath.Join(readonlyDir, "subdir", "cannot-write.sarif")

	err := saveSARIFToFile(log, outPath)
	if err == nil {
		t.Error("Expected error when writing to unwritable directory")
	}
}

// ============================================
// Coverage: ensureRule deduplication (line 199)
// ============================================

// ============================================
// Coverage: resolveExistingPath
// ============================================

func TestResolveExistingPath_ExistingPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	resolved := resolveExistingPath(tmpDir)
	// Should resolve to the real path (EvalSymlinks on existing dir)
	expected, _ := filepath.EvalSymlinks(tmpDir)
	if resolved != expected {
		t.Errorf("resolveExistingPath(%q) = %q, want %q", tmpDir, resolved, expected)
	}
}

func TestResolveExistingPath_NonExistentFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nonexistent", "file.sarif")
	resolved := resolveExistingPath(target)
	// Parent doesn't exist, grandparent (tmpDir) does.
	// Should resolve tmpDir's real path + "nonexistent/file.sarif"
	resolvedTmp, _ := filepath.EvalSymlinks(tmpDir)
	expected := filepath.Join(resolvedTmp, "nonexistent", "file.sarif")
	if resolved != expected {
		t.Errorf("resolveExistingPath(%q) = %q, want %q", target, resolved, expected)
	}
}

func TestResolveExistingPath_SymlinkInPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetDir := t.TempDir()

	// Create a symlink inside tmpDir pointing to targetDir
	symlinkPath := filepath.Join(tmpDir, "link")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks: %v", err)
	}

	// Resolve a file path through the symlink
	filePath := filepath.Join(symlinkPath, "output.sarif")
	resolved := resolveExistingPath(filePath)

	// The resolved path should be under targetDir, not under tmpDir
	resolvedTarget, _ := filepath.EvalSymlinks(targetDir)
	expected := filepath.Join(resolvedTarget, "output.sarif")
	if resolved != expected {
		t.Errorf("resolveExistingPath(%q) = %q, want %q (should follow symlink)", filePath, resolved, expected)
	}
}

func TestSaveSARIFToFile_SymlinkResolution(t *testing.T) {
	t.Parallel()
	// Verify that saveSARIFToFile resolves symlinks before checking allowed paths.
	// On macOS, t.TempDir() dirs are all under the OS temp dir, so symlinks
	// between temp dirs are legitimately allowed by the temp dir check.
	// This test verifies that symlink resolution works correctly by checking
	// that the file is written to the RESOLVED target (not the symlink path).
	cwdDir := t.TempDir()
	targetDir := t.TempDir()

	symlinkPath := filepath.Join(cwdDir, "link")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks: %v", err)
	}

	// Verify resolveExistingPath follows the symlink correctly
	filePath := filepath.Join(symlinkPath, "result.sarif")
	resolvedFile := resolveExistingPath(filePath)

	resolvedTarget, _ := filepath.EvalSymlinks(targetDir)
	expected := filepath.Join(resolvedTarget, "result.sarif")
	if resolvedFile != expected {
		t.Errorf("resolveExistingPath through symlink: got %q, want %q", resolvedFile, expected)
	}

	// The resolved path is under the OS temp dir, so the write should succeed.
	// This validates that the resolution + allowed-dir check work together.
	log := &SARIFLog{Version: "2.1.0", Schema: "test", Runs: []sarifRun{}}
	err := saveSARIFToFile(log, filePath)
	if err != nil {
		t.Fatalf("saveSARIFToFile through symlink under temp should succeed: %v", err)
	}

	// Verify the file was written to the resolved target
	resolvedTargetFile := filepath.Join(resolvedTarget, "result.sarif")
	if _, err := os.Stat(resolvedTargetFile); os.IsNotExist(err) {
		t.Error("Expected file to be written at resolved symlink target")
	}
}

func TestSaveSARIFToFile_OutsideAllowedDirs(t *testing.T) {
	t.Parallel()
	// Test that paths outside both cwd and temp dir are rejected.
	log := &SARIFLog{Version: "2.1.0", Schema: "test", Runs: []sarifRun{}}
	err := saveSARIFToFile(log, "/nonexistent/path/evil.sarif")
	if err == nil {
		t.Error("Expected error for path outside allowed directories")
	}
	if err != nil && !strings.Contains(err.Error(), "save_to path must be under") {
		t.Errorf("Expected 'save_to path must be under' error, got: %v", err)
	}
}
