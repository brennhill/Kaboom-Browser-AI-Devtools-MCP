// Purpose: Implements top-level stop/force commands that orchestrate daemon shutdown strategies.
// Why: Keeps command flow readable while platform/process mechanics live in dedicated helpers.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

const (
	stopPollInterval             = 100 * time.Millisecond
	stopHTTPShutdownTimeout      = 3 * time.Second
	stopProcessLookupSettleDelay = 500 * time.Millisecond
)

// runStopMode gracefully stops a running server on the specified port.
// Uses hybrid approach: PID file (fast) -> HTTP /shutdown (graceful) -> platform-aware process kill (fallback).
func runStopMode(port int) {
	diag.Printf("Stopping kaboom server on port %d...\n", port)
	logCommandInvocation("stop_command_invoked", "kaboom --stop", port)

	if stopViaPIDFile(port) {
		return
	}
	if stopViaHTTP(port) {
		return
	}
	stopViaProcessLookup(port)
}

// runForceCleanup kills ALL running kaboom daemons across all ports.
// Used during package install to ensure clean upgrade from older versions.
func runForceCleanup() {
	diag.Println("Force cleanup: Killing all running kaboom daemons...")

	logFile := resolveLogFile()
	cleanupEntry := map[string]any{
		"type":       "lifecycle",
		"event":      "force_cleanup_invoked",
		"source":     "kaboom --force",
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONLogEntry(logFile, cleanupEntry)

	var killed, failedToKill int
	if runtime.GOOS != "windows" {
		killed, failedToKill = killUnixKaboomProcesses()
	} else {
		killed = killWindowsKaboomProcesses()
	}

	cleanupPIDFiles()
	printForceCleanupSummary(killed, failedToKill)
}

func logCommandInvocation(event string, source string, port int) {
	logFile := resolveLogFile()
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      event,
		"port":       port,
		"source":     source,
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONLogEntry(logFile, entry)
}

func resolveLogFile() string {
	logFile, err := state.DefaultLogFile()
	if err != nil {
		if legacy, legacyErr := state.LegacyDefaultLogFile(); legacyErr == nil {
			return legacy
		}
		return filepath.Join(os.TempDir(), "kaboom.jsonl")
	}
	return logFile
}

func writeJSONLogEntry(logFile string, entry map[string]any) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	_ = os.MkdirAll(filepath.Dir(logFile), 0o750)
	// #nosec G304 -- log file path resolved from trusted runtime state directory
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		return
	}
	_, _ = f.Write(data)
	_, _ = f.Write([]byte{'\n'})
	_ = f.Close()
}

func stopViaPIDFile(port int) bool {
	pid := procctl.ReadPIDFile(port)
	if pid <= 0 || !procctl.IsProcessAlive(pid) {
		return false
	}
	diag.Printf("Found server (PID %d) via PID file\n", pid)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return false
	}
	diag.Printf("Sent SIGTERM to PID %d\n", pid)
	for i := 0; i < 20; i++ {
		time.Sleep(stopPollInterval)
		if !procctl.IsProcessAlive(pid) {
			diag.Println("Server stopped successfully")
			procctl.RemovePIDFile(port)
			return true
		}
	}
	diag.Println("Server did not exit within 2 seconds, sending SIGKILL")
	_ = process.Kill()
	procctl.RemovePIDFile(port)
	diag.Println("Server killed")
	return true
}

func stopViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: stopHTTPShutdownTimeout}
	req, _ := http.NewRequest("POST", shutdownURL, nil)
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close() // lint:body-close-ok immediate close on success path
		diag.Println("Server stopped via HTTP endpoint")
		procctl.RemovePIDFile(port)
		return true
	}
	if resp != nil {
		_ = resp.Body.Close() // lint:body-close-ok immediate close before fallback path
	}
	return false
}

func stopViaProcessLookup(port int) {
	diag.Println("Trying process lookup fallback...")
	pids, findErr := procctl.FindProcessOnPort(port)
	if findErr != nil || len(pids) == 0 {
		diag.Printf("No server found on port %d\n", port)
		procctl.RemovePIDFile(port)
		return
	}
	for _, pidNum := range pids {
		diag.Printf("Sending termination signal to PID %d\n", pidNum)
		_ = procctl.KillProcessByPID(pidNum)
	}
	time.Sleep(stopProcessLookupSettleDelay)
	if !bridge.IsServerRunning(port) {
		diag.Println("Server stopped successfully")
		procctl.RemovePIDFile(port)
	} else {
		diag.Printf("Server may still be running, try: %s\n", procctl.PortKillHintForce(port))
	}
}
