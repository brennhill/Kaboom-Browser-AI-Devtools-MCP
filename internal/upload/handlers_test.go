// Purpose: Handler-level integration tests that the security chain is actually wired into Stage 1/3.
// Docs: docs/features/feature/file-upload/index.md

// handlers_test.go — Stage 1/3 handler checks that exercise uploadsec through the handler surface.
package upload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// testSecurityWithDir returns a Security scoped to a specific directory.
func testSecurityWithDir(t *testing.T, dir string) *uploadsec.Security {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("testSecurityWithDir: EvalSymlinks(%s) failed: %v", dir, err)
	}
	return uploadsec.NewSecurity(resolved, nil)
}

func TestSecurity_FileRead_DeniedPath(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("SECRET=abc"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	sec := uploadsec.NewSecurity("", nil)
	resp := HandleFileRead(FileReadRequest{FilePath: envFile}, sec, false)
	if resp.Success {
		t.Error("reading .env file should be denied")
	}
	if !strings.Contains(resp.Error, ".env") {
		t.Errorf("error should mention .env, got: %s", resp.Error)
	}
}

func TestSecurity_FormSubmit_OutsideUploadDir(t *testing.T) {
	uploadDir := t.TempDir()
	otherDir := t.TempDir()
	f := filepath.Join(otherDir, "data.txt")
	if err := os.WriteFile(f, []byte("test"), 0o644); err != nil {
		t.Fatalf("write data.txt: %v", err)
	}

	sec := testSecurityWithDir(t, uploadDir)
	resp := HandleFormSubmit(FormSubmitRequest{
		FormAction:    "https://example.com/upload",
		FileInputName: "file",
		FilePath:      f,
	}, sec)

	if resp.Success {
		t.Error("file outside upload-dir should fail for Stage 3")
	}
	if !strings.Contains(resp.Error, "outside") {
		t.Errorf("error should mention outside upload dir, got: %s", resp.Error)
	}
}
