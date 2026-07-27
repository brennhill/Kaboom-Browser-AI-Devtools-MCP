// Purpose: Contract tests for upload endpoint behavior.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/osauto"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func TestSmokeUploadScripts_Stage4PathsAligned(t *testing.T) {
	repoRoot := repoRootFromTestFile(t)

	bootstrapPath := filepath.Join(repoRoot, "scripts", "smoke-tests", "01-bootstrap.sh")
	uploadPath := filepath.Join(repoRoot, "scripts", "smoke-tests", "15-file-upload.sh")

	bootstrapRaw, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", bootstrapPath, err)
	}
	uploadRaw, err := os.ReadFile(uploadPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", uploadPath, err)
	}

	bootstrap := string(bootstrapRaw)
	uploadScript := string(uploadRaw)

	// If bootstrap starts daemon with Stage 4 enabled and no explicit --upload-dir,
	// smoke upload fixtures must live under the daemon default: ~/kaboom-upload-dir.
	usesDefaultUploadDir := strings.Contains(bootstrap, "start_daemon_with_flags --enable-os-upload-automation") &&
		!strings.Contains(bootstrap, "--upload-dir=")
	if !usesDefaultUploadDir {
		t.Skip("bootstrap uses explicit --upload-dir or no Stage 4 default mode; contract check not applicable")
	}

	re := regexp.MustCompile(`(?m)^UPLOAD_TEST_DIR="([^"]+)"`)
	match := re.FindStringSubmatch(uploadScript)
	if len(match) != 2 {
		t.Fatalf("UPLOAD_TEST_DIR assignment not found in %s", uploadPath)
	}
	uploadDirExpr := match[1]
	if !strings.HasPrefix(uploadDirExpr, "${HOME}/kaboom-upload-dir/") {
		t.Fatalf("smoke upload dir (%q) is outside default Stage 4 upload dir (${HOME}/kaboom-upload-dir); Stage 4 tests will fail with 'outside allowed upload directory'", uploadDirExpr)
	}
}

func TestOSAutomation_RejectsSmokeTmpPathOutsideDefaultUploadDir(t *testing.T) {
	home := t.TempDir()
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err == nil {
		home = resolvedHome
	}

	allowedDir := filepath.Join(home, "kaboom-upload-dir")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatalf("failed to create allowed dir: %v", err)
	}

	smokeFile := filepath.Join(home, ".kaboom", "tmp", "smoke-upload-12345", "upload-15-16.txt")
	if err := os.MkdirAll(filepath.Dir(smokeFile), 0o755); err != nil {
		t.Fatalf("failed to create smoke tmp dir: %v", err)
	}
	if err := os.WriteFile(smokeFile, []byte("smoke upload"), 0o644); err != nil {
		t.Fatalf("failed to write smoke file: %v", err)
	}

	sec, secErr := uploadsec.ValidateUploadDir(allowedDir, nil)
	if secErr != nil {
		t.Fatalf("ValidateUploadDir() error = %v", secErr)
	}
	resp := osauto.HandleOSAutomation(upload.OSAutomationInjectRequest{
		FilePath:   smokeFile,
		BrowserPID: 12345, // skip PID auto-detect in test
	}, sec)

	if resp.Success {
		t.Fatal("expected Stage 4 to reject file outside allowed upload directory")
	}
	if resp.Stage != 4 {
		t.Fatalf("expected stage=4, got %d", resp.Stage)
	}
	if !strings.Contains(resp.Error, "outside the allowed upload directory") {
		t.Fatalf("expected outside-upload-dir error, got: %s", resp.Error)
	}
	if !strings.Contains(resp.Error, allowedDir) {
		t.Fatalf("expected error to mention allowed dir %q, got: %s", allowedDir, resp.Error)
	}
}

func TestSmokeUploadScript_Stage4PollTreatsCompleteWithErrorAsFailed(t *testing.T) {
	repoRoot := repoRootFromTestFile(t)
	uploadPath := filepath.Join(repoRoot, "scripts", "smoke-tests", "15-file-upload.sh")

	raw, err := os.ReadFile(uploadPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", uploadPath, err)
	}
	s := string(raw)

	// Contract: in Stage 4 helper, a poll payload that says status=complete but
	// contains FAILED/error text must be marked failed, not complete.
	if !strings.Contains(s, "_upload_and_poll_stage4") {
		t.Fatalf("missing _upload_and_poll_stage4 helper in %s", uploadPath)
	}
	if !strings.Contains(s, `grep -qi 'FAILED —\|\"error\":'`) {
		t.Fatalf("Stage 4 poll helper must detect FAILED/error payloads inside status=complete responses")
	}
	if !strings.Contains(s, `UPLOAD_FINAL_STATUS="failed"`) {
		t.Fatalf("Stage 4 poll helper must set UPLOAD_FINAL_STATUS=failed for complete+error payloads")
	}
}

func TestSmokeUploadScript_15_17HandlesAccessibilityAsSkip(t *testing.T) {
	repoRoot := repoRootFromTestFile(t)
	uploadPath := filepath.Join(repoRoot, "scripts", "smoke-tests", "15-file-upload.sh")

	raw, err := os.ReadFile(uploadPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", uploadPath, err)
	}
	s := string(raw)

	// Contract: 15.17 should skip known environment blockers (Accessibility/xdotool)
	// the same way 15.16 does, instead of reporting product failure.
	const fnStart = "run_test_15_17() {"
	const fnEnd = "\n}\nrun_test_15_17"
	start := strings.Index(s, fnStart)
	end := strings.Index(s, fnEnd)
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("missing run_test_15_17 in %s", uploadPath)
	}
	block := s[start:end]
	if !strings.Contains(block, `skip "Stage 4 needs macOS Accessibility permission`) {
		t.Fatalf("15.17 must skip when macOS Accessibility is missing")
	}
}

func TestSmokeUploadScript_15_15HasFallbackWhenExecuteJSReturnsNoResult(t *testing.T) {
	repoRoot := repoRootFromTestFile(t)
	uploadPath := filepath.Join(repoRoot, "scripts", "smoke-tests", "15-file-upload.sh")

	raw, err := os.ReadFile(uploadPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", uploadPath, err)
	}
	s := string(raw)

	// Contract: 15.15 should not hard-fail as UNKNOWN when execute_js returns
	// success with no result payload; it should fall back to upload completion data.
	if !strings.Contains(s, `if [ "$js_result" = "UNKNOWN" ]; then`) {
		t.Fatalf("15.15 must have UNKNOWN fallback branch for execute_js result parsing")
	}
	if !strings.Contains(s, `UPLOAD_FINAL_TEXT`) {
		t.Fatalf("15.15 fallback must inspect UPLOAD_FINAL_TEXT from upload polling")
	}
	if !strings.Contains(s, `execute_js returned no result`) {
		t.Fatalf("15.15 fallback should emit explicit pass message when upload completion proves file reached input")
	}
}

func repoRootFromTestFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// cmd/browser-agent/<this_file>.go -> repo root is ../..
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}
