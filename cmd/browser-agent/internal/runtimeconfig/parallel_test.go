// parallel_test.go — Verifies deterministic isolated state directories for parallel daemons.

package runtimeconfig

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestApplyParallelStateDirCreatesDeterministicIsolation(t *testing.T) {
	root := t.TempDir()
	t.Setenv(state.StateDirEnv, root)
	resolved, warnings, err := ApplyParallelStateDir(true, "", time.Unix(123, 456), 77)
	if err != nil {
		t.Fatalf("ApplyParallelStateDir: %v", err)
	}
	want := filepath.Join(root, "parallel", "run-123000000456-77")
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], resolved) {
		t.Fatalf("warnings = %q, want generated path", warnings)
	}
}

func TestApplyParallelStateDirPreservesExplicitOrDisabledState(t *testing.T) {
	now := time.Unix(123, 0)
	explicit := filepath.Join(t.TempDir(), "isolated")
	resolved, warnings, err := ApplyParallelStateDir(true, explicit, now, 77)
	if err != nil || resolved != explicit || len(warnings) != 0 {
		t.Fatalf("explicit = (%q, %q, %v)", resolved, warnings, err)
	}
	resolved, warnings, err = ApplyParallelStateDir(false, "", now, 77)
	if err != nil || resolved != "" || len(warnings) != 0 {
		t.Fatalf("disabled = (%q, %q, %v)", resolved, warnings, err)
	}
}
