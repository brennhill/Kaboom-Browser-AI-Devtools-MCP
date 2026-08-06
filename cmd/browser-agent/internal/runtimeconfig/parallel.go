// parallel.go — Owns isolated runtime state setup for parallel daemon launches.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package runtimeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// ApplyParallelStateDir creates and activates an isolated state directory when required.
func ApplyParallelStateDir(parallel bool, stateDir string, now time.Time, pid int) (string, []string, error) {
	if !parallel || strings.TrimSpace(stateDir) != "" {
		return stateDir, nil, nil
	}
	root, err := state.RootDir()
	if err != nil {
		return "", nil, fmt.Errorf("cannot resolve runtime state root: %w", err)
	}
	generated := filepath.Join(root, "parallel", fmt.Sprintf("run-%d-%d", now.UnixNano(), pid))
	if err := os.MkdirAll(generated, 0o750); err != nil {
		return "", nil, fmt.Errorf("cannot create parallel state dir %q: %w", generated, err)
	}
	resolved := filepath.Clean(generated)
	if err := os.Setenv(state.StateDirEnv, resolved); err != nil {
		return "", nil, fmt.Errorf("failed to set %s: %w", state.StateDirEnv, err)
	}
	return resolved, []string{fmt.Sprintf("parallel_mode_state_dir_auto: %s", resolved)}, nil
}
