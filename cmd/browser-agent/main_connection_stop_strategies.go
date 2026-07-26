// Purpose: Concrete shutdown strategies for stop/force-cleanup commands.
// Why: Separates platform/process termination mechanics from top-level command flow.

package main

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
)

const (
	// stopPollInterval is the interval between process-alive checks when waiting
	// for a server to exit after SIGTERM.
	stopPollInterval = 100 * time.Millisecond

	// stopHTTPShutdownTimeout is the HTTP client timeout for the /shutdown endpoint.
	stopHTTPShutdownTimeout = 3 * time.Second

	// stopProcessLookupSettleDelay is the pause after sending termination signals
	// via process lookup before checking whether the server actually stopped.
	stopProcessLookupSettleDelay = 500 * time.Millisecond
)

// stopViaPIDFile attempts to stop the server using the PID file (fast path).
// Returns true if the server was stopped successfully.
func stopViaPIDFile(port int) bool {
	pid := procctl.ReadPIDFile(port)
	if pid <= 0 || !procctl.IsProcessAlive(pid) {
		return false
	}

	fmt.Printf("Found server (PID %d) via PID file\n", pid)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return false
	}
	fmt.Printf("Sent SIGTERM to PID %d\n", pid)

	for i := 0; i < 20; i++ {
		time.Sleep(stopPollInterval)
		if !procctl.IsProcessAlive(pid) {
			fmt.Println("Server stopped successfully")
			procctl.RemovePIDFile(port)
			return true
		}
	}

	fmt.Println("Server did not exit within 2 seconds, sending SIGKILL")
	_ = process.Kill()
	procctl.RemovePIDFile(port)
	fmt.Println("Server killed")
	return true
}

// stopViaHTTP attempts to stop the server using the /shutdown HTTP endpoint.
// Returns true if the server acknowledged the shutdown.
func stopViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: stopHTTPShutdownTimeout}
	req, _ := http.NewRequest("POST", shutdownURL, nil)
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close() // lint:body-close-ok immediate close on success path
		fmt.Println("Server stopped via HTTP endpoint")
		procctl.RemovePIDFile(port)
		return true
	}
	if resp != nil {
		_ = resp.Body.Close() // lint:body-close-ok immediate close before fallback path
	}
	return false
}

// stopViaProcessLookup finds processes on the port and terminates them.
func stopViaProcessLookup(port int) {
	fmt.Println("Trying process lookup fallback...")
	pids, findErr := procctl.FindProcessOnPort(port)
	if findErr != nil || len(pids) == 0 {
		fmt.Printf("No server found on port %d\n", port)
		procctl.RemovePIDFile(port)
		return
	}

	for _, pidNum := range pids {
		fmt.Printf("Sending termination signal to PID %d\n", pidNum)
		_ = procctl.KillProcessByPID(pidNum)
	}

	time.Sleep(stopProcessLookupSettleDelay)
	if !bridge.IsServerRunning(port) {
		fmt.Println("Server stopped successfully")
		procctl.RemovePIDFile(port)
	} else {
		fmt.Printf("Server may still be running, try: %s\n", procctl.PortKillHintForce(port))
	}
}
