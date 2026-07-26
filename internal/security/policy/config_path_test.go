// config_path_test.go — Tests for security config path resolution.
// Purpose: Tests for security config path resolution.
// Docs: docs/features/feature/security-hardening/index.md

package policy

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestGetConfigPathUsesStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	original := configPath()
	setConfigPath("")
	defer setConfigPath(original)

	got := configPath()
	want := filepath.Join(stateRoot, "security", "security.json")
	if got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}

func TestAddToWhitelistErrorIncludesResolvedConfigPath(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("MCP_MODE", "1")

	original := configPath()
	setConfigPath("")
	defer setConfigPath(original)

	InitMode()
	err := AddToWhitelist("https://cdn.example.com")
	if err == nil {
		t.Fatal("AddToWhitelist() error = nil, want blocked in MCP mode")
	}

	wantPath := filepath.Join(stateRoot, "security", "security.json")
	if !strings.Contains(err.Error(), wantPath) {
		t.Fatalf("AddToWhitelist() error = %q, want it to include %q", err.Error(), wantPath)
	}
}
