// port.go — Platform-specific port-kill hints, process lookup, and PID extraction for error recovery messages.
// Why: Generates actionable OS-native commands (lsof, netstat, taskkill) in error output across macOS/Linux/Windows.

package procctl

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// PortKillHint returns a platform-appropriate command to kill a process on the given port.
// On macOS/Linux: lsof -ti :<port> | xargs kill
// On Windows: netstat -ano | findstr :<port>  then  taskkill /F /PID <pid>
func PortKillHint(port int) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("netstat -ano | findstr :%d  then  taskkill /F /PID <pid>", port)
	}
	return fmt.Sprintf("lsof -ti :%d | xargs kill", port)
}

// PortKillHintForce returns a platform-appropriate forceful kill command.
// On macOS/Linux: kill -9 $(lsof -ti :<port>)
// On Windows: netstat -ano | findstr :<port>  then  taskkill /F /PID <pid>
func PortKillHintForce(port int) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("netstat -ano | findstr :%d  then  taskkill /F /PID <pid>", port)
	}
	return fmt.Sprintf("kill -9 $(lsof -ti :%d)", port)
}

// parseNetstatPIDs extracts PIDs from Windows netstat output for a given port.
// Format: TCP  0.0.0.0:7890  0.0.0.0:0  LISTENING  <PID>
func parseNetstatPIDs(output string, port int) []int {
	var pids []int
	needle := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, needle) || !strings.Contains(strings.ToUpper(line), "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			if pid, err := strconv.Atoi(fields[len(fields)-1]); err == nil && pid > 0 {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

// parseLsofPIDs extracts PIDs from lsof output (one PID per line).
func parseLsofPIDs(output string) []int {
	var pids []int
	for _, p := range strings.Split(strings.TrimSpace(output), "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if pid, err := strconv.Atoi(p); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// FindProcessOnPort returns the PIDs of processes listening on the given port.
// Uses lsof on macOS/Linux and netstat on Windows.
func FindProcessOnPort(port int) ([]int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("netstat", "-ano")
	} else {
		cmd = exec.Command("lsof", "-tiTCP:"+strconv.Itoa(port), "-sTCP:LISTEN")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("find process on port %d: %w", port, err)
	}

	if runtime.GOOS == "windows" {
		return parseNetstatPIDs(string(output), port), nil
	}
	return parseLsofPIDs(string(output)), nil
}

// GetProcessCommand returns the command line of a process by PID.
// Uses ps on macOS/Linux and wmic/tasklist on Windows.
func GetProcessCommand(pid int) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	} else {
		cmd = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
	}

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(output))
	if runtime.GOOS == "windows" {
		// tasklist CSV format: "name.exe","PID","Session Name","Session#","Mem Usage"
		// Extract just the process name
		parts := strings.Split(result, ",")
		if len(parts) >= 1 {
			return strings.Trim(parts[0], "\"")
		}
	}
	return result
}

// KillProcessByPID sends a termination signal to a process.
// Uses SIGTERM on macOS/Linux and os.Process.Kill on Windows.
func KillProcessByPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if runtime.GOOS == "windows" {
		// Windows doesn't support SIGTERM; use Kill directly
		return process.Kill()
	}
	return process.Signal(syscall.SIGTERM)
}
