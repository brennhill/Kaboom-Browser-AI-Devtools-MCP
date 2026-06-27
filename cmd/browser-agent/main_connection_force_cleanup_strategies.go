// Purpose: Force-cleanup process termination helpers for cross-port daemon cleanup.
// Why: Keeps broad process sweep and summary reporting separate from graceful single-port stop strategies.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

const (
	// terminateSignalSettleDelay is the pause after SIGTERM before checking if the
	// process exited, allowing it time to shut down gracefully.
	terminateSignalSettleDelay = 100 * time.Millisecond
)

var forceCleanupCommandNames = []string{"kaboom", "gasoline", "strum"}

// killUnixKaboomProcesses finds and kills Kaboom plus legacy processes on Unix systems
// using lsof and pkill. Returns (killed, failedToKill) counts.
func killUnixKaboomProcesses() (int, int) {
	killed := 0
	failedToKill := 0
	selfPID := os.Getpid()

	for _, commandName := range forceCleanupCommandNames {
		cmd := exec.Command("lsof", "-c", commandName)
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, pid := range lsofListedPIDs(string(output), selfPID) {
			k, f := terminateProcess(pid)
			killed += k
			failedToKill += f
		}
	}

	// Also try pkill as fallback.
	for _, pattern := range []string{"kaboom.*--daemon", "gasoline.*--daemon", "strum.*--daemon"} {
		pkillCmd := exec.Command("pkill", "-f", pattern)
		_ = pkillCmd.Run()
	}

	return killed, failedToKill
}

// lsofListedPIDs parses `lsof -c <name>` output into candidate PIDs.
// selfPID is excluded: `lsof -c kaboom` prefix-matches the running binary's
// own command name, and force-cleanup must never signal itself.
func lsofListedPIDs(output string, selfPID int) []int {
	pids := []int{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 || pid == selfPID {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

// terminateProcess sends SIGTERM then SIGKILL to a process. Returns (killed, failed) counts.
func terminateProcess(pid int) (int, int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, 0
	}
	if err := process.Signal(syscall.SIGTERM); err == nil {
		fmt.Printf("  Sent SIGTERM to PID %d\n", pid)
		time.Sleep(terminateSignalSettleDelay)
		if !isProcessAlive(pid) {
			return 1, 0
		}
	}
	if err := process.Kill(); err == nil {
		fmt.Printf("  Sent SIGKILL to PID %d\n", pid)
		return 1, 0
	}
	return 0, 1
}

// killWindowsKaboomProcesses kills Kaboom plus legacy processes on Windows using taskkill.
func killWindowsKaboomProcesses() int {
	killed := 0
	for _, imageName := range []string{"kaboom.exe", "gasoline.exe", "strum.exe"} {
		cmd := exec.Command("taskkill", "/IM", imageName, "/F")
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "SUCCESS") || strings.Contains(line, "terminated") {
				killed++
			}
		}
	}
	return killed
}

// cleanupPIDFiles removes PID files for common port range.
func cleanupPIDFiles() {
	ports := []int{17890}
	for p := 7890; p <= 7910; p++ {
		ports = append(ports, p)
	}
	for _, p := range ports {
		removePIDFile(p)
		removeLegacyPIDVariants(p)
	}
}

func killUnixKaboomProcessesQuietly() (int, int) {
	for _, pattern := range []string{"kaboom.*--daemon", "gasoline.*--daemon", "strum.*--daemon"} {
		cmd := exec.Command("pkill", "-f", pattern)
		_ = cmd.Run() //nolint:errcheck // best-effort process cleanup; exit code irrelevant
	}
	return 0, 0
}

func killWindowsKaboomProcessesQuietly() int {
	for _, imageName := range []string{"kaboom.exe", "gasoline.exe", "strum.exe"} {
		cmd := exec.Command("taskkill", "/IM", imageName, "/F")
		_ = cmd.Run() //nolint:errcheck // best-effort process cleanup; exit code irrelevant
	}
	return 0
}

func removeLegacyPIDVariants(port int) {
	homeDir, _ := os.UserHomeDir()
	roots := []string{}
	if stateRoot, err := state.RootDir(); err == nil && strings.TrimSpace(stateRoot) != "" {
		roots = append(roots, filepath.Join(stateRoot, "run"))
	}
	if homeDir != "" {
		roots = append(roots,
			filepath.Join(homeDir, ".kaboom", "run"),
			filepath.Join(homeDir, ".gasoline", "run"),
			filepath.Join(homeDir, ".strum", "run"),
		)
	}
	if xdgStateHome := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdgStateHome != "" {
		roots = append(roots,
			filepath.Join(xdgStateHome, "kaboom", "run"),
			filepath.Join(xdgStateHome, "gasoline", "run"),
			filepath.Join(xdgStateHome, "strum", "run"),
		)
	}

	for _, root := range roots {
		_ = os.Remove(filepath.Join(root, "kaboom-"+strconv.Itoa(port)+".pid"))
		_ = os.Remove(filepath.Join(root, "gasoline-"+strconv.Itoa(port)+".pid"))
		_ = os.Remove(filepath.Join(root, "strum-"+strconv.Itoa(port)+".pid"))
	}
	if homeDir == "" {
		return
	}
	_ = os.Remove(filepath.Join(homeDir, ".kaboom-"+strconv.Itoa(port)+".pid"))
	_ = os.Remove(filepath.Join(homeDir, ".gasoline-"+strconv.Itoa(port)+".pid"))
	_ = os.Remove(filepath.Join(homeDir, ".strum-"+strconv.Itoa(port)+".pid"))
}

// printForceCleanupSummary outputs the results of the force cleanup operation.
func printForceCleanupSummary(killed, failedToKill int) {
	fmt.Println()
	if killed > 0 {
		fmt.Printf("✓ Successfully killed %d Kaboom/legacy process(es)\n", killed)
	}
	if failedToKill > 0 {
		fmt.Printf("⚠ Failed to kill %d process(es) (may have already exited)\n", failedToKill)
	}
	if killed == 0 && failedToKill == 0 {
		fmt.Println("✓ No running Kaboom/legacy processes found")
	}
	fmt.Println()
	fmt.Println("Cleaned up PID files. Safe to proceed with installation.")
}

func runForceCleanupQuietly() error {
	if runtime.GOOS != "windows" {
		_, _ = killUnixKaboomProcessesQuietly()
	} else {
		_ = killWindowsKaboomProcessesQuietly()
	}
	cleanupPIDFiles()
	return nil
}
