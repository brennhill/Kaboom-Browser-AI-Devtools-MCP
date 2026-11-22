//go:build windows
// +build windows

// Purpose: Configures detached-process spawn attributes for Windows daemon child processes.
// Why: Enables persistent background process launch without tying child lifetime to console session state.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package util

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcess configures the command to run as a detached process
func SetDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
