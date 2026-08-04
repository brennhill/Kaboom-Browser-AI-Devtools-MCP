// Purpose: Handles daemon recycle and zombie-process recovery during bridge startup.
// Why: Isolates recovery mechanics from connection orchestration and retry logic.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/nativeinstall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
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
		procctl.RemovePIDFile(port)
		return true
	}

	pid := procctl.ReadPIDFile(port)
	if pid > 0 && pid != os.Getpid() {
		terminatePIDQuiet(pid, false)
	}

	pids, err := procctl.FindProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, false)
		}
	}

	if waitForPortRelease(port, recoverySlowPortWait) {
		procctl.RemovePIDFile(port)
		return true
	}

	pids, err = procctl.FindProcessOnPort(port)
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
		procctl.RemovePIDFile(port)
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
		if !bridgeRunner.IsServerRunning(port) {
			return true
		}
		time.Sleep(recoveryPortPollInterval)
	}
	return !bridgeRunner.IsServerRunning(port)
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
	if procctl.IsProcessAlive(pid) {
		_ = process.Kill()
	}
}

// The injectable process/port seams. They wrap the primitives defined above (and
// in main_helpers_pid.go / platform_errors.go), so they live here, next to those
// implementations, rather than in the daemonlife package that also consumes them:
// daemonlife owns the single-instance POLICY, this package owns the mechanics.
// reclaimPort and identifyPortHolder below use them directly; daemonlife receives
// them through daemonlifeDeps below.
var (
	// daemonProcessCommand looks up a PID's command line.
	daemonProcessCommand = procctl.GetProcessCommand
	// daemonIsProcessAlive reports whether a PID is still running.
	daemonIsProcessAlive = procctl.IsProcessAlive
	// daemonIsServerRunning reports whether something is accepting on a port.
	daemonIsServerRunning = bridgeRunner.IsServerRunning
	// daemonTryShutdown asks the daemon on a port to shut down over HTTP.
	daemonTryShutdown = tryShutdownViaHTTP
	// daemonWaitForPortRelease blocks until a port is free or the timeout elapses.
	daemonWaitForPortRelease = waitForPortRelease
	// daemonTerminatePID signals a PID (SIGTERM, or SIGKILL when force is set).
	daemonTerminatePID = terminatePIDQuiet
	// daemonFindProcessOnPort lists the PIDs holding a port.
	daemonFindProcessOnPort = procctl.FindProcessOnPort
)

// ourDaemonBinaryNames are the binary names this project ships or builds. A
// leftover daemon may come from a different install path or an older version, so
// matching only our own absolute executable path would miss the very zombies this
// reclaim exists to clear.
var ourDaemonBinaryNames = []string{"kaboom-agentic-browser", "browser-agent"}

// processLooksLikeOurDaemon reports whether `cmdline` is one of OUR daemons.
//
// This is deliberately an allow-list on the executable, not a heuristic on the
// whole command line: the argument vector can contain anything (a port number, a
// path), and matching on that would let an unrelated process be claimed as ours.
// An empty cmdline (ps failed, or the process is already gone) is NOT ours — if we
// cannot prove ownership we must not kill.
func processLooksLikeOurDaemon(cmdline, ownExecPath string) bool {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return false
	}
	// Only the executable (argv[0]) decides; arguments are never consulted.
	exe := cmdline
	if i := strings.IndexByte(exe, ' '); i >= 0 {
		exe = exe[:i]
	}
	if ownExecPath != "" && exe == ownExecPath {
		return true
	}
	base := filepath.Base(exe)
	if ownExecPath != "" && base == filepath.Base(ownExecPath) {
		return true
	}
	for _, name := range ourDaemonBinaryNames {
		// `go test` runs the package binary as "<name>.test", which is still ours.
		if base == name || base == name+".test" || base == name+".exe" {
			return true
		}
	}
	return false
}

// isOurDaemonPID reports whether pid is one of our daemons, per the command-line
// allow-list above. Unknown => false (never kill what we cannot identify).
func isOurDaemonPID(pid int) bool {
	ownExec, err := os.Executable()
	if err != nil {
		ownExec = ""
	}
	return processLooksLikeOurDaemon(daemonProcessCommand(pid), ownExec)
}

// reclaimPort clears anything already listening on `port` so daemon startup is
// deterministically single-instance. It finds the owning PID(s), logs them,
// terminates them (SIGTERM then SIGKILL), and waits for release. This is the
// recovery for a port blocked by a LEFTOVER process the lock-based takeover did
// not cover — a zombie from an un-clean restart or crash. That is exactly the
// condition that leaves the terminal port (main+1) unreachable and the terminal
// dead. Returns true if the port is free afterward. Self is never killed.
//
// Only OUR daemons are ever terminated. The owning PIDs come from `lsof`, which
// reports whoever holds the port — previously every one of them was SIGTERM'd and
// then SIGKILL'd with a self-PID check as the only guard, so an unrelated program
// the user happened to run on 7890/7891 (a dev server, a database) was killed on
// startup, and the terminal supervisor repeated that before each restart attempt.
// A process we cannot positively identify as ours is left alone and logged.
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
		if !isOurDaemonPID(pid) {
			// Someone else's process holds the port. Killing it would be destructive
			// and is never our call — log loudly so "the port is busy" is diagnosable
			// instead of silently doing nothing (rule 25).
			cmdline := daemonProcessCommand(pid)
			server.logLifecycle("port_reclaim_skipped_foreign", port, map[string]any{
				"purpose": purpose, "owner_pid": pid, "owner_command": cmdline,
			})
			diag.Printf("[Kaboom] port %d is held by another process (pid %d: %s) — not reclaiming it. "+
				"Free that port or start Kaboom on a different one.\n", port, pid, cmdline)
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

// identifyPortHolder returns the PID and command line of whatever is listening on
// port, or (0, "") if that cannot be determined. Best effort and never fatal: it
// exists purely to turn "port busy" into something the user can act on.
func identifyPortHolder(port int) (int, string) {
	owners, err := daemonFindProcessOnPort(port)
	if err != nil || len(owners) == 0 {
		return 0, ""
	}
	self := os.Getpid()
	for _, pid := range owners {
		if pid <= 0 || pid == self {
			continue
		}
		return pid, daemonProcessCommand(pid)
	}
	return 0, ""
}

// lifecycleLogger adapts Server to daemonlife.Logger.
type lifecycleLogger struct{ server *Server }

func (l lifecycleLogger) LogLifecycle(event string, port int, fields map[string]any) {
	l.server.logLifecycle(event, port, fields)
}

// daemonlifeDeps binds the recovery process and port seams to daemonlife.
func daemonlifeDeps(server *Server) daemonlife.Deps {
	return daemonlife.Deps{
		Log:       lifecycleLogger{server: server},
		Version:   version,
		Warnf:     diag.Printf,
		Recovery:  server.stateRecovery,
		Incidents: server.incidents,

		IsProcessAlive:     daemonIsProcessAlive,
		IsServerRunning:    daemonIsServerRunning,
		TryShutdown:        daemonTryShutdown,
		WaitForPortRelease: daemonWaitForPortRelease,
		TerminatePID:       daemonTerminatePID,
		FetchHealth:        fetchDaemonHealth,
		ReadPIDFile:        procctl.ReadPIDFile,
		RemovePIDFile:      procctl.RemovePIDFile,
	}
}

// fetchDaemonHealth reduces a health probe to the facts the takeover policy needs.
func fetchDaemonHealth(ctx context.Context, port int, timeout time.Duration) (reachable bool, version string, refused bool) {
	h := nativeinstall.FetchHealth(ctx, port, timeout)
	return h.Reachable, h.Version, h.Refused
}
