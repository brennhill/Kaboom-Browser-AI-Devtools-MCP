// reclaimer.go — Owns safe daemon identification, port reclaim, and lifecycle policy seams.

package daemonrecovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	corebridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

type host struct {
	processCommand     func(int) string
	isProcessAlive     func(int) bool
	isServerRunning    func(int) bool
	tryShutdown        func(int) bool
	waitForPortRelease func(int, time.Duration) bool
	terminatePID       func(int, bool)
	findProcessOnPort  func(int) ([]int, error)
	readPIDFile        func(int) int
	removePIDFile      func(int) error
}

// Config supplies process-wide diagnostics and lifecycle state to a Reclaimer.
type Config struct {
	Version      string
	Recovery     statediag.Reporter
	Incidents    *incident.Store
	LogLifecycle func(event string, port int, fields map[string]any)
	Diagnosticf  func(format string, args ...any)
}

// Reclaimer owns all process and port mechanics used during daemon takeover.
type Reclaimer struct {
	host         host
	config       Config
	lifecycleLog daemonlife.Logger
}

type lifecycleLogger struct {
	log func(event string, port int, fields map[string]any)
}

func (logger lifecycleLogger) LogLifecycle(event string, port int, fields map[string]any) {
	logger.log(event, port, fields)
}

// New constructs the canonical daemon recovery owner.
func New(config Config) *Reclaimer {
	if config.Diagnosticf == nil {
		config.Diagnosticf = diag.Printf
	}
	if config.LogLifecycle == nil {
		config.LogLifecycle = func(event string, port int, _ map[string]any) {
			config.Diagnosticf("[Kaboom] lifecycle event %s on port %d had no structured log owner\n", event, port)
		}
	}
	return &Reclaimer{
		host: host{
			processCommand: procctl.GetProcessCommand, isProcessAlive: procctl.IsProcessAlive,
			isServerRunning: corebridge.IsServerRunning, tryShutdown: tryShutdownViaHTTP,
			waitForPortRelease: waitForPortRelease, terminatePID: terminatePIDQuiet,
			findProcessOnPort: procctl.FindProcessOnPort,
			readPIDFile:       procctl.ReadPIDFile,
			removePIDFile: func(port int) error {
				path := procctl.PIDFilePath(port)
				if path == "" {
					return nil
				}
				return os.Remove(path)
			},
		},
		config:       config,
		lifecycleLog: lifecycleLogger{log: config.LogLifecycle},
	}
}

// CleanupStalePIDFile removes obsolete process identity without mistaking PID
// reuse for ownership of the daemon port.
func (r *Reclaimer) CleanupStalePIDFile(port int) error {
	pid := r.host.readPIDFile(port)
	if pid <= 0 {
		// EXPECTED_ABSENCE: a first start has no PID file, and an invalid record
		// cannot identify a process safely enough to emit an ownership incident.
		return nil
	}
	if r.host.isProcessAlive(pid) {
		ownerPIDs, err := r.host.findProcessOnPort(port)
		if err != nil {
			r.config.LogLifecycle("stale_pid_port_lookup_failed", port, map[string]any{"stale_pid": pid, "error": err.Error()})
		} else {
			for _, ownerPID := range ownerPIDs {
				if ownerPID == pid {
					r.config.LogLifecycle("port_conflict_detected", port, map[string]any{"existing_pid": pid})
					return fmt.Errorf("port %d already in use by PID %d (run 'kaboom --stop --port %d' to stop it)", port, pid, port)
				}
			}
			r.config.LogLifecycle("stale_pid_owner_mismatch", port, map[string]any{"stale_pid": pid, "owner_pids": ownerPIDs})
		}
	} else {
		r.config.LogLifecycle("stale_pid_removed", port, map[string]any{"stale_pid": pid})
	}
	if err := r.host.removePIDFile(port); err != nil && !os.IsNotExist(err) {
		r.config.LogLifecycle("stale_pid_remove_failed", port, map[string]any{"stale_pid": pid, "error": err.Error()})
	}
	return nil
}

// LifecycleDeps returns the complete process seam consumed by daemonlife policy.
func (r *Reclaimer) LifecycleDeps() daemonlife.Deps {
	return daemonlife.Deps{
		Log: r.lifecycleLog, Version: r.config.Version, Warnf: r.config.Diagnosticf,
		Recovery: r.config.Recovery, Incidents: r.config.Incidents,
		IsProcessAlive: r.host.isProcessAlive, IsServerRunning: r.host.isServerRunning,
		TryShutdown: r.host.tryShutdown, WaitForPortRelease: r.host.waitForPortRelease,
		TerminatePID: r.host.terminatePID, FetchHealth: fetchDaemonHealth,
		ReadPIDFile: procctl.ReadPIDFile, RemovePIDFile: procctl.RemovePIDFile,
	}
}

// ReclaimPort terminates only positively identified Kaboom daemons holding port.
func (r *Reclaimer) ReclaimPort(port int, purpose string) bool {
	owners, err := r.host.findProcessOnPort(port)
	if err != nil || len(owners) == 0 {
		return !r.host.isServerRunning(port)
	}
	self := os.Getpid()
	killed := make([]int, 0, len(owners))
	for _, pid := range owners {
		if pid <= 0 || pid == self {
			continue
		}
		if !r.isOurDaemonPID(pid) {
			command := r.host.processCommand(pid)
			r.config.LogLifecycle("port_reclaim_skipped_foreign", port, map[string]any{
				"purpose": purpose, "owner_pid": pid, "owner_command": command,
			})
			r.config.Diagnosticf("[Kaboom] port %d is held by another process (pid %d: %s) — not reclaiming it. "+
				"Free that port or start Kaboom on a different one.\n", port, pid, command)
			continue
		}
		r.config.LogLifecycle("port_reclaim_terminating", port, map[string]any{"purpose": purpose, "owner_pid": pid})
		r.host.terminatePID(pid, false)
		killed = append(killed, pid)
	}
	if len(killed) == 0 {
		return !r.host.isServerRunning(port)
	}
	if !r.host.waitForPortRelease(port, 2*time.Second) {
		for _, pid := range killed {
			r.host.terminatePID(pid, true)
		}
		r.host.waitForPortRelease(port, 2*time.Second)
	}
	freed := !r.host.isServerRunning(port)
	r.config.LogLifecycle("port_reclaimed", port, map[string]any{"purpose": purpose, "killed_pids": killed, "freed": freed})
	return freed
}

// IdentifyPortHolder reports the first valid non-self process holding port.
func (r *Reclaimer) IdentifyPortHolder(port int) (int, string) {
	owners, err := r.host.findProcessOnPort(port)
	if err != nil || len(owners) == 0 {
		return 0, ""
	}
	self := os.Getpid()
	for _, pid := range owners {
		if pid > 0 && pid != self {
			return pid, r.host.processCommand(pid)
		}
	}
	return 0, ""
}

func (r *Reclaimer) isOurDaemonPID(pid int) bool {
	ownExecutable, err := os.Executable()
	if err != nil {
		ownExecutable = ""
	}
	return processLooksLikeOurDaemon(r.host.processCommand(pid), ownExecutable)
}

func processLooksLikeOurDaemon(commandLine, ownExecutable string) bool {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return false
	}
	executable := commandLine
	if index := strings.IndexByte(executable, ' '); index >= 0 {
		executable = executable[:index]
	}
	if ownExecutable != "" && (executable == ownExecutable || filepath.Base(executable) == filepath.Base(ownExecutable)) {
		return true
	}
	base := filepath.Base(executable)
	for _, name := range [...]string{"kaboom-agentic-browser", "browser-agent"} {
		if base == name || base == name+".test" || base == name+".exe" {
			return true
		}
	}
	return false
}
