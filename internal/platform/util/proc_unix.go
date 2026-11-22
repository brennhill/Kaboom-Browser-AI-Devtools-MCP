//go:build !windows
// +build !windows

// Purpose: Configures detached-process spawn attributes for Unix daemon child processes.
// Why: Enables daemonized subprocess lifecycle behavior without inheriting parent process groups.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package util

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcess configures the command to run as a detached process
func SetDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
