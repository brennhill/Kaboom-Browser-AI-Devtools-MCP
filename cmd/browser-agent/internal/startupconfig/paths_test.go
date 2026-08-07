// paths_test.go — Tests deterministic startup path and upload configuration.

package startupconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestBuildUploadSecurityCreatesDefaultBoundary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	security, err := BuildUploadSecurity(false, "", []string{"private-*"})
	if err != nil || security == nil {
		t.Fatalf("BuildUploadSecurity() = %#v, %v", security, err)
	}
	if _, err := os.Stat(filepath.Join(home, "kaboom-upload-dir")); err != nil {
		t.Fatalf("default upload directory: %v", err)
	}
}

func TestNormalizeStateDirPublishesAbsolutePath(t *testing.T) {
	t.Setenv(state.StateDirEnv, "")
	resolved, err := NormalizeStateDir(filepath.Join(".", "test-state"))
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(resolved) || os.Getenv(state.StateDirEnv) != resolved {
		t.Fatalf("resolved state dir = %q env=%q", resolved, os.Getenv(state.StateDirEnv))
	}
	if empty, err := NormalizeStateDir(""); err != nil || empty != "" {
		t.Fatalf("empty state dir = %q, %v", empty, err)
	}
}

func TestResolveLogFileUsesStateDefault(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	resolved, warning := ResolveLogFile("")
	if resolved == "" || !strings.HasSuffix(resolved, "kaboom.jsonl") || warning != "" {
		t.Fatalf("ResolveLogFile() = %q, %q", resolved, warning)
	}
	if explicit, warning := ResolveLogFile("custom.jsonl"); explicit != "custom.jsonl" || warning != "" {
		t.Fatalf("explicit log file = %q, %q", explicit, warning)
	}
}
