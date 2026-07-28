// Purpose: Tests for lint-hardening rules and static checks.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// lint_hardening_test.go — Go test wrapper for custom lint rules.
// Runs scripts/lint-hardening.sh as a Go test so violations are caught
// by `go test` (including `go test -short`). Fast: only grep-based scans.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// projectRoot returns the repository root by navigating from this source file.
func projectRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../cmd/browser-agent/lint_hardening_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// TestLintHardening runs the custom hardening lint script and fails the test
// if any violations are found. The script checks for bare goroutines, unchecked
// JSON encodes, missing headers, route sync, middleware, SafeGo closures, and
// queue overflow logging.
func TestLintHardening(t *testing.T) {
	t.Parallel()

	root := projectRoot()
	scriptPath := filepath.Join(root, "scripts", "lint-hardening.sh")

	cmd := exec.Command("bash", scriptPath) // #nosec G204 -- fixed script path, test-only
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lint-hardening.sh failed (exit %v):\n%s", err, output)
	}
	t.Logf("lint-hardening.sh passed:\n%s", output)
}

func TestRootDoesNotReexportCanonicalTypes(t *testing.T) {
	rootFiles, err := filepath.Glob(filepath.Join(projectRoot(), "cmd", "browser-agent", "*.go"))
	if err != nil {
		t.Fatalf("list browser-agent root files: %v", err)
	}
	for _, forbidden := range []string{
		"type JSONRPCRequest =",
		"type JSONRPCResponse =",
		"type MCPToolResult =",
		"type Annotation =",
		"type AnnotationStore =",
		"type ToolCallLimiter =",
		"var NewToolCallLimiter =",
		"var randomInt63 =",
		"var legacyMCPServerNames =",
	} {
		for _, path := range rootFiles {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s re-exports canonical API %q", filepath.Base(path), forbidden)
			}
		}
	}
}

func TestPackagesDoNotReexportCanonicalLogEntry(t *testing.T) {
	for _, relativePath := range []string{
		"internal/mcp/types.go",
		"internal/server/main_handlers.go",
		"internal/pagination/pagination_logs.go",
		"internal/security/scan/types.go",
		"cmd/browser-agent/internal/logstore/store.go",
	} {
		path := filepath.Join(projectRoot(), relativePath)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		for _, forbidden := range []string{"type LogEntry =", "type Entry ="} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s re-exports canonical log contract with %q", relativePath, forbidden)
			}
		}
	}
}
