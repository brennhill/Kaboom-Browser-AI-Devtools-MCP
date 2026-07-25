// Purpose: Handles daemon recycle and zombie-process recovery during bridge startup.
// Why: Isolates recovery mechanics from connection orchestration and retry logic.

package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
)

const (
	// recoveryFastPortWait is the initial wait for port release after HTTP shutdown.
	recoveryFastPortWait = 500 * time.Millisecond

	// recoverySlowPortWait is the extended wait for port release after SIGTERM/SIGKILL escalation.
	recoverySlowPortWait = 1500 * time.Millisecond

	// recoveryShutdownHTTPTimeout is the HTTP client timeout for the one-shot shutdown probe.
	recoveryShutdownHTTPTimeout = 500 * time.Millisecond

	// recoveryPortPollInterval is the polling interval when waiting for a port to be released.
	recoveryPortPollInterval = 50 * time.Millisecond

	// recoveryTermGracePeriod is the pause after SIGTERM before escalating to SIGKILL.
	recoveryTermGracePeriod = 200 * time.Millisecond
)

func stopServerForUpgrade(port int) bool {
	_ = tryShutdownViaHTTP(port)
	if waitForPortRelease(port, recoveryFastPortWait) {
		removePIDFile(port)
		return true
	}

	pid := readPIDFile(port)
	if pid > 0 && pid != os.Getpid() {
		terminatePIDQuiet(pid, false)
	}

	pids, err := findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, false)
		}
	}

	if waitForPortRelease(port, recoverySlowPortWait) {
		removePIDFile(port)
		return true
	}

	pids, err = findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, true)
		}
	}

	released := waitForPortRelease(port, recoverySlowPortWait)
	if released {
		removePIDFile(port)
	}
	return released
}

func tryShutdownViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: recoveryShutdownHTTPTimeout}
	req, _ := http.NewRequest(http.MethodPost, shutdownURL, nil)
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err != nil {
		return false
	}
	_ = resp.Body.Close() // lint:body-close-ok one-shot shutdown probe
	return resp.StatusCode == http.StatusOK
}

func waitForPortRelease(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !bridge.IsServerRunning(port) {
			return true
		}
		time.Sleep(recoveryPortPollInterval)
	}
	return !bridge.IsServerRunning(port)
}

func terminatePIDQuiet(pid int, force bool) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if force {
		_ = process.Kill()
		return
	}

	if runtime.GOOS == "windows" {
		_ = process.Kill()
		return
	}

	_ = process.Signal(syscall.SIGTERM)
	time.Sleep(recoveryTermGracePeriod)
	if isProcessAlive(pid) {
		_ = process.Kill()
	}
}

// reclaimPort clears anything already listening on `port` so daemon startup is
// deterministically single-instance. It finds the owning PID(s), logs them,
// terminates them (SIGTERM then SIGKILL), and waits for release. This is the
// recovery for a port blocked by a LEFTOVER process the lock-based takeover did
// not cover — a zombie from an un-clean restart or crash. That is exactly the
// condition that leaves the terminal port (main+1) unreachable and the terminal
// dead. Returns true if the port is free afterward. Self is never killed.
func reclaimPort(server *Server, port int, purpose string) bool {
	owners, err := daemonFindProcessOnPort(port)
	if err != nil || len(owners) == 0 {
		return !daemonIsServerRunning(port)
	}
	self := os.Getpid()
	killed := make([]int, 0, len(owners))
	for _, pid := range owners {
		if pid <= 0 || pid == self {
			continue
		}
		server.logLifecycle("port_reclaim_terminating", port, map[string]any{"purpose": purpose, "owner_pid": pid})
		daemonTerminatePID(pid, false)
		killed = append(killed, pid)
	}
	if len(killed) == 0 {
		return !daemonIsServerRunning(port)
	}
	if !daemonWaitForPortRelease(port, 2*time.Second) {
		for _, pid := range killed {
			daemonTerminatePID(pid, true) // force
		}
		daemonWaitForPortRelease(port, 2*time.Second)
	}
	freed := !daemonIsServerRunning(port)
	server.logLifecycle("port_reclaimed", port, map[string]any{"purpose": purpose, "killed_pids": killed, "freed": freed})
	return freed
}
