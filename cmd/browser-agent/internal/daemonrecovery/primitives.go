// primitives.go — Performs bounded daemon shutdown, process termination, and health probes.

package daemonrecovery

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	_ = tryShutdownViaHTTP(port)
	if waitForPortRelease(port, fastPortWait) {
		procctl.RemovePIDFile(port)
		return true
	}

	pid := procctl.ReadPIDFile(port)
	if pid > 0 && pid != os.Getpid() {
		terminatePIDQuiet(pid, false)
	}

	pids, err := procctl.FindProcessOnPort(port)
	if err == nil {
		for _, candidate := range pids {
			if candidate != os.Getpid() {
				terminatePIDQuiet(candidate, false)
			}
		}
	}

	if waitForPortRelease(port, slowPortWait) {
		procctl.RemovePIDFile(port)
		return true
	}

	pids, err = procctl.FindProcessOnPort(port)
	if err == nil {
		for _, candidate := range pids {
			if candidate != os.Getpid() {
				terminatePIDQuiet(candidate, true)
			}
		}
	}

	released := waitForPortRelease(port, slowPortWait)
	if released {
		procctl.RemovePIDFile(port)
	}
	return released
}

func tryShutdownViaHTTP(port int) bool {
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

func waitForPortRelease(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !corebridge.IsServerRunning(port) {
			return true
		}
		time.Sleep(portPollInterval)
	}
	return !corebridge.IsServerRunning(port)
}

func terminatePIDQuiet(pid int, force bool) {
	// Adapts the canonical terminator to daemonlife's no-error seam. The error is
	// deliberately not returned here because every caller already verifies the
	// OUTCOME it cares about — WaitForPortRelease — and escalates on its own; a
	// signal that "succeeded" while the port stayed held would be the misleading
	// result, not this one.
	_ = procctl.TerminatePID(pid, force)
}

func fetchDaemonHealth(ctx context.Context, port int, timeout time.Duration) (reachable bool, version string, refused bool) {
	health := nativeinstall.FetchHealth(ctx, port, timeout)
	return health.Reachable, health.Version, health.Refused
}
