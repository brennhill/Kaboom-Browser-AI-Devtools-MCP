// primitives.go — Performs bounded daemon shutdown, process termination, and health probes.

package daemonrecovery

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/nativeinstall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	corebridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
)

const (
	fastPortWait        = 500 * time.Millisecond
	slowPortWait        = 1500 * time.Millisecond
	shutdownHTTPTimeout = 500 * time.Millisecond
	portPollInterval    = 50 * time.Millisecond
	termGracePeriod     = 200 * time.Millisecond
)

// StopServerForUpgrade applies bounded graceful and forced shutdown escalation.
func StopServerForUpgrade(port int) bool {
	_ = TryShutdownViaHTTP(port)
	if WaitForPortRelease(port, fastPortWait) {
		procctl.RemovePIDFile(port)
		return true
	}

	pid := procctl.ReadPIDFile(port)
	if pid > 0 && pid != os.Getpid() {
		TerminatePIDQuiet(pid, false)
	}

	pids, err := procctl.FindProcessOnPort(port)
	if err == nil {
		for _, candidate := range pids {
			if candidate != os.Getpid() {
				TerminatePIDQuiet(candidate, false)
			}
		}
	}

	if WaitForPortRelease(port, slowPortWait) {
		procctl.RemovePIDFile(port)
		return true
	}

	pids, err = procctl.FindProcessOnPort(port)
	if err == nil {
		for _, candidate := range pids {
			if candidate != os.Getpid() {
				TerminatePIDQuiet(candidate, true)
			}
		}
	}

	released := WaitForPortRelease(port, slowPortWait)
	if released {
		procctl.RemovePIDFile(port)
	}
	return released
}

// TryShutdownViaHTTP requests a graceful shutdown from a loopback daemon.
func TryShutdownViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: shutdownHTTPTimeout}
	req, err := http.NewRequest(http.MethodPost, shutdownURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err != nil {
		return false
	}
	_ = resp.Body.Close() // lint:body-close-ok one-shot shutdown probe
	return resp.StatusCode == http.StatusOK
}

// WaitForPortRelease polls until the loopback daemon port is no longer accepting connections.
func WaitForPortRelease(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !corebridge.IsServerRunning(port) {
			return true
		}
		time.Sleep(portPollInterval)
	}
	return !corebridge.IsServerRunning(port)
}

// TerminatePIDQuiet requests termination and escalates after a bounded grace period.
func TerminatePIDQuiet(pid int, force bool) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if force || runtime.GOOS == "windows" {
		_ = process.Kill()
		return
	}

	_ = process.Signal(syscall.SIGTERM)
	time.Sleep(termGracePeriod)
	if procctl.IsProcessAlive(pid) {
		_ = process.Kill()
	}
}

// FetchDaemonHealth reduces a health response to the takeover policy contract.
func FetchDaemonHealth(ctx context.Context, port int, timeout time.Duration) (reachable bool, version string, refused bool) {
	health := nativeinstall.FetchHealth(ctx, port, timeout)
	return health.Reachable, health.Version, health.Refused
}
