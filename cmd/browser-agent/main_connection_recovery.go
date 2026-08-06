// Purpose: Handles daemon recycle and zombie-process recovery during bridge startup.
// Why: Isolates recovery mechanics from connection orchestration and retry logic.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonrecovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	corebridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
)

// The injectable process/port seams. They wrap the primitives defined above (and
// in main_helpers_pid.go / platform_errors.go), so they live here, next to those
// implementations, rather than in the daemonlife package that also consumes them:
// daemonlife owns the single-instance POLICY, this package owns the mechanics.
// reclaimPort and identifyPortHolder below use them directly; daemonlife receives
// them through daemonlifeDeps below.
type daemonHost struct {
	processCommand     func(int) string
	isProcessAlive     func(int) bool
	isServerRunning    func(int) bool
	tryShutdown        func(int) bool
	waitForPortRelease func(int, time.Duration) bool
	terminatePID       func(int, bool)
	findProcessOnPort  func(int) ([]int, error)
}

func newDaemonHost() daemonHost {
	return daemonHost{
		processCommand: procctl.GetProcessCommand, isProcessAlive: procctl.IsProcessAlive,
		isServerRunning: corebridge.IsServerRunning, tryShutdown: daemonrecovery.TryShutdownViaHTTP,
		waitForPortRelease: daemonrecovery.WaitForPortRelease, terminatePID: daemonrecovery.TerminatePIDQuiet,
		findProcessOnPort: procctl.FindProcessOnPort,
	}
}

// ourDaemonBinaryNames are the binary names this project ships or builds. A
// leftover daemon may come from a different install path or an older version, so
// matching only our own absolute executable path would miss the very zombies this
// reclaim exists to clear.
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
	for _, name := range [...]string{"kaboom-agentic-browser", "browser-agent"} {
		// `go test` runs the package binary as "<name>.test", which is still ours.
		if base == name || base == name+".test" || base == name+".exe" {
			return true
		}
	}
	return false
}

// isOurDaemonPID reports whether pid is one of our daemons, per the command-line
// allow-list above. Unknown => false (never kill what we cannot identify).
func isOurDaemonPID(host daemonHost, pid int) bool {
	ownExec, err := os.Executable()
	if err != nil {
		ownExec = ""
	}
	return processLooksLikeOurDaemon(host.processCommand(pid), ownExec)
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
	host := server.daemonHost
	owners, err := host.findProcessOnPort(port)
	if err != nil || len(owners) == 0 {
		return !host.isServerRunning(port)
	}
	self := os.Getpid()
	killed := make([]int, 0, len(owners))
	for _, pid := range owners {
		if pid <= 0 || pid == self {
			continue
		}
		if !isOurDaemonPID(host, pid) {
			// Someone else's process holds the port. Killing it would be destructive
			// and is never our call — log loudly so "the port is busy" is diagnosable
			// instead of silently doing nothing (rule 25).
			cmdline := host.processCommand(pid)
			server.logLifecycle("port_reclaim_skipped_foreign", port, map[string]any{
				"purpose": purpose, "owner_pid": pid, "owner_command": cmdline,
			})
			diag.Printf("[Kaboom] port %d is held by another process (pid %d: %s) — not reclaiming it. "+
				"Free that port or start Kaboom on a different one.\n", port, pid, cmdline)
			continue
		}
		server.logLifecycle("port_reclaim_terminating", port, map[string]any{"purpose": purpose, "owner_pid": pid})
		host.terminatePID(pid, false)
		killed = append(killed, pid)
	}
	if len(killed) == 0 {
		return !host.isServerRunning(port)
	}
	if !host.waitForPortRelease(port, 2*time.Second) {
		for _, pid := range killed {
			host.terminatePID(pid, true) // force
		}
		host.waitForPortRelease(port, 2*time.Second)
	}
	freed := !host.isServerRunning(port)
	server.logLifecycle("port_reclaimed", port, map[string]any{"purpose": purpose, "killed_pids": killed, "freed": freed})
	return freed
}

// identifyPortHolder returns the PID and command line of whatever is listening on
// port, or (0, "") if that cannot be determined. Best effort and never fatal: it
// exists purely to turn "port busy" into something the user can act on.
func identifyPortHolder(host daemonHost, port int) (int, string) {
	owners, err := host.findProcessOnPort(port)
	if err != nil || len(owners) == 0 {
		return 0, ""
	}
	self := os.Getpid()
	for _, pid := range owners {
		if pid <= 0 || pid == self {
			continue
		}
		return pid, host.processCommand(pid)
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

		IsProcessAlive:     server.daemonHost.isProcessAlive,
		IsServerRunning:    server.daemonHost.isServerRunning,
		TryShutdown:        server.daemonHost.tryShutdown,
		WaitForPortRelease: server.daemonHost.waitForPortRelease,
		TerminatePID:       server.daemonHost.terminatePID,
		FetchHealth:        daemonrecovery.FetchDaemonHealth,
		ReadPIDFile:        procctl.ReadPIDFile,
		RemovePIDFile:      procctl.RemovePIDFile,
	}
}
