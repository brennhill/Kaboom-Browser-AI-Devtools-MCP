// Purpose: Covers SARIF path, rule, node, and package-boundary edge cases.
// Docs: docs/features/feature/sarif-export/index.md

package sarif

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read SARIF package: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			count++
		}
	}
	if count > 10 {
		t.Fatalf("SARIF package has %d Go files; maximum is 10", count)
	}
}

func TestIsPathUnderResolvedDir_NonexistentDir(t *testing.T) {
	t.Parallel()
	if isPathUnderResolvedDir("/tmp/some/file.sarif", "/nonexistent/dir/that/does/not/exist") {
		t.Error("expected false for non-existent directory")
	}
}

func TestValidateSARIFSavePath_TempDirPath(t *testing.T) {
	t.Parallel()
	resolvedTmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Skipf("cannot resolve temp dir: %v", err)
	}
	path := filepath.Join(resolvedTmp, "kaboom-test", "output.sarif")
	if err := validateSARIFSavePath(path, path); err != nil {
		t.Fatalf("validate temp path: %v", err)
	}
}

func TestEnsureRule_NoWCAGTags(t *testing.T) {
	t.Parallel()
	run := &sarifRun{Tool: sarifTool{Driver: sarifDriver{Rules: []sarifRule{}}}, Results: []sarifResult{}}
	violation := axeViolation{ID: "test-rule", Description: "Test description", Help: "Test help", Tags: []string{"cat.aria", "TTv5"}}
	if index := ensureRule(run, make(map[string]int), violation); index != 0 {
		t.Fatalf("rule index = %d, want 0", index)
	}
	if run.Tool.Driver.Rules[0].Properties != nil {
		t.Fatal("rule without WCAG tags has properties")
	}
}

func TestNodeToResult_EmptyTarget(t *testing.T) {
	t.Parallel()
	result := nodeToResult(axeViolation{ID: "test", Help: "Help text"}, axeNode{HTML: "<div></div>"}, 0, "error")
	if uri := result.Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "" {
		t.Fatalf("empty target URI = %q", uri)
	}
}
