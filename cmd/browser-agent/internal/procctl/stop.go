// stop.go — Stops daemon processes and removes canonical PID state.

package procctl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

const (
	stopPollInterval             = 100 * time.Millisecond
	stopHTTPShutdownTimeout      = 3 * time.Second
	stopProcessLookupSettleDelay = 500 * time.Millisecond
	terminateSignalSettleDelay   = 100 * time.Millisecond
)

var forceCleanupCommandNames = []string{"kaboom", "gasoline", "strum"}

// Stop gracefully stops a running server on the specified port.
// Uses hybrid approach: PID file (fast) -> HTTP /shutdown (graceful) -> platform-aware process kill (fallback).
func Stop(port int, isServerRunning func(int) bool) {
	diag.Printf("Stopping kaboom server on port %d...\n", port)
	logCommandInvocation("stop_command_invoked", "kaboom --stop", port)

	if stopViaPIDFile(port) {
		return
	}
	if stopViaHTTP(port) {
		return
	}
	stopViaProcessLookup(port, isServerRunning)
}

// ForceCleanup kills ALL running kaboom daemons across all ports.
// Used during package install to ensure clean upgrade from older versions.
func ForceCleanup() {
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

	CleanupPIDFiles()
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
		return ""
	}
	return logFile
}

func writeJSONLogEntry(logFile string, entry map[string]any) {
	if logFile == "" {
		return
	}
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
	pid := ReadPIDFile(port)
	if pid <= 0 || !IsProcessAlive(pid) {
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
		if !IsProcessAlive(pid) {
			diag.Println("Server stopped successfully")
			RemovePIDFile(port)
			return true
		}
	}
	diag.Println("Server did not exit within 2 seconds, sending SIGKILL")
	_ = process.Kill()
	RemovePIDFile(port)
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
		RemovePIDFile(port)
		return true
	}
	if resp != nil {
		_ = resp.Body.Close() // lint:body-close-ok immediate close before fallback path
	}
	return false
}

func stopViaProcessLookup(port int, isServerRunning func(int) bool) {
	diag.Println("Trying process lookup fallback...")
	pids, findErr := FindProcessOnPort(port)
	if findErr != nil || len(pids) == 0 {
		diag.Printf("No server found on port %d\n", port)
		RemovePIDFile(port)
		return
	}
	for _, pidNum := range pids {
		diag.Printf("Sending termination signal to PID %d\n", pidNum)
		_ = KillProcessByPID(pidNum)
	}
	time.Sleep(stopProcessLookupSettleDelay)
	if !isServerRunning(port) {
		diag.Println("Server stopped successfully")
		RemovePIDFile(port)
	} else {
		diag.Printf("Server may still be running, try: %s\n", PortKillHintForce(port))
	}
}

func killUnixKaboomProcesses() (int, int) {
	killed, failedToKill := 0, 0
	selfPID := os.Getpid()
	for _, commandName := range forceCleanupCommandNames {
		output, err := exec.Command("lsof", "-c", commandName).Output()
		if err != nil {
			continue
		}
		for _, pid := range lsofListedPIDs(string(output), selfPID) {
			k, failed := terminateProcess(pid)
			killed += k
			failedToKill += failed
		}
	}
	for _, pattern := range []string{"kaboom.*--daemon", "gasoline.*--daemon", "strum.*--daemon"} {
		_ = exec.Command("pkill", "-f", pattern).Run()
	}
	return killed, failedToKill
}

func lsofListedPIDs(output string, selfPID int) []int {
	pids := []int{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err == nil && pid > 0 && pid != selfPID {
			pids = append(pids, pid)
		}
	}
	return pids
}

func terminateProcess(pid int) (int, int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, 0
	}
	if err := process.Signal(syscall.SIGTERM); err == nil {
		diag.Printf("  Sent SIGTERM to PID %d\n", pid)
		time.Sleep(terminateSignalSettleDelay)
		if !IsProcessAlive(pid) {
			return 1, 0
		}
	}
	if err := process.Kill(); err == nil {
		diag.Printf("  Sent SIGKILL to PID %d\n", pid)
		return 1, 0
	}
	return 0, 1
}

func killWindowsKaboomProcesses() int {
	killed := 0
	for _, imageName := range []string{"kaboom.exe", "gasoline.exe", "strum.exe"} {
		output, err := exec.Command("taskkill", "/IM", imageName, "/F").CombinedOutput()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, "SUCCESS") || strings.Contains(line, "terminated") {
				killed++
			}
		}
	}
	return killed
}

func CleanupPIDFiles() {
	ports := []int{17890}
	for port := 7890; port <= 7910; port++ {
		ports = append(ports, port)
	}
	for _, port := range ports {
		RemovePIDFile(port)
	}
}

func killUnixKaboomProcessesQuietly() (int, int) {
	for _, pattern := range []string{"kaboom.*--daemon", "gasoline.*--daemon", "strum.*--daemon"} {
		_ = exec.Command("pkill", "-f", pattern).Run()
	}
	return 0, 0
}

func killWindowsKaboomProcessesQuietly() int {
	for _, imageName := range []string{"kaboom.exe", "gasoline.exe", "strum.exe"} {
		_ = exec.Command("taskkill", "/IM", imageName, "/F").Run()
	}
	return 0
}

func printForceCleanupSummary(killed, failedToKill int) {
	diag.Println()
	if killed > 0 {
		diag.Printf("✓ Successfully killed %d Kaboom/legacy process(es)\n", killed)
	}
	if failedToKill > 0 {
		diag.Printf("⚠ Failed to kill %d process(es) (may have already exited)\n", failedToKill)
	}
	if killed == 0 && failedToKill == 0 {
		diag.Println("✓ No running Kaboom/legacy processes found")
	}
	diag.Println()
	diag.Println("Cleaned up PID files. Safe to proceed with installation.")
}

func ForceCleanupQuietly() error {
	if runtime.GOOS != "windows" {
		_, _ = killUnixKaboomProcessesQuietly()
	} else {
		_ = killWindowsKaboomProcessesQuietly()
	}
	CleanupPIDFiles()
	return nil
}
