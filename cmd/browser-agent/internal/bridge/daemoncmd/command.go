// command.go — Builds a detached persistent daemon process for bridge startup.
// Docs: docs/features/feature/lazy-server-start/index.md

package daemoncmd

import (
	"fmt"
	"os"
	"os/exec"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Build constructs the daemon command with persistent stdio and runtime options.
func Build(port int, logFile string, maxEntries int, processArgv0 func(string) string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find executable: %w. Verify kaboom is installed correctly", err)
	}

	args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
	if stateDir := os.Getenv(statecfg.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}
	if logFile != "" {
		args = append(args, "--log-file", logFile)
	}
	if maxEntries > 0 {
		args = append(args, "--max-entries", fmt.Sprintf("%d", maxEntries))
	}
	cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path with fixed flags
	cmd.Args[0] = processArgv0(exe)
	// nil streams are connected to os.DevNull by os/exec. A non-file writer
	// creates a bridge-owned pipe that can terminate the daemon after bridge exit.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)
	return cmd, nil
}
