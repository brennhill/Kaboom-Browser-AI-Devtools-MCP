// Purpose: Tests for parallel configuration access safety.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestApplyParallelModeStateDir_AutoGeneratesWhenMissing(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	stateDir := ""
	var warnings []string
	if err := applyParallelModeStateDir(true, &stateDir, &warnings); err != nil {
		t.Fatalf("applyParallelModeStateDir() error = %v", err)
	}
	if strings.TrimSpace(stateDir) == "" {
		t.Fatal("stateDir should be auto-generated in parallel mode")
	}
	if !strings.HasPrefix(stateDir, filepath.Join(stateRoot, "parallel")+string(filepath.Separator)) {
		t.Fatalf("stateDir = %q, want prefix %q", stateDir, filepath.Join(stateRoot, "parallel")+string(filepath.Separator))
	}
}

func TestApplyParallelModeStateDir_PreservesExplicitStateDir(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "isolated")
	stateDir := explicit
	var warnings []string
	if err := applyParallelModeStateDir(true, &stateDir, &warnings); err != nil {
		t.Fatalf("applyParallelModeStateDir() error = %v", err)
	}
	if stateDir != explicit {
		t.Fatalf("stateDir = %q, want explicit %q", stateDir, explicit)
	}
}

func TestApplyParallelModeStateDir_NoopWhenDisabled(t *testing.T) {
	stateDir := ""
	var warnings []string
	if err := applyParallelModeStateDir(false, &stateDir, &warnings); err != nil {
		t.Fatalf("applyParallelModeStateDir() error = %v", err)
	}
	if stateDir != "" {
		t.Fatalf("stateDir = %q, want unchanged empty string", stateDir)
	}
}
