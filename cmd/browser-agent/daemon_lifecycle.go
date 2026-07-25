// Purpose: Manages daemon lock file, process liveness checks, and stale-daemon cleanup for singleton enforcement.
// Why: Prevents port conflicts and zombie daemons by coordinating lifecycle via PID-based lock records.

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type daemonLaunchOptions struct {
	Parallel bool
}

type daemonLockRecord struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	StateDir  string `json:"state_dir"`
	Version   string `json:"version,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

var (
	daemonIsProcessAlive     = isProcessAlive
	daemonIsServerRunning    = bridge.IsServerRunning
	daemonTryShutdown        = tryShutdownViaHTTP
	daemonWaitForPortRelease = waitForPortRelease
	daemonTerminatePID       = terminatePIDQuiet
	daemonNow                = time.Now
	daemonFindProcessOnPort  = findProcessOnPort
)

func enforceDaemonStartupPolicy(server *Server, port int, opts daemonLaunchOptions) error {
	stateDir, err := state.RootDir()
	if err != nil {
		return fmt.Errorf("cannot resolve state dir: %w", err)
	}
	rec, err := readDaemonLockFile()
	if err != nil {
		return err
	}
	if rec == nil {
		return nil
	}

	if opts.Parallel {
		return validateParallelIsolation(rec)
	}
	return performDefaultTakeover(server, stateDir, port, rec)
}

func validateParallelIsolation(rec *daemonLockRecord) error {
	if rec.PID <= 0 || rec.Port <= 0 {
		return fmt.Errorf(
			"parallel mode requires isolated --state-dir; invalid daemon lock metadata (pid=%d, port=%d) at %s",
			rec.PID,
			rec.Port,
			daemonLockFilePathForError(),
		)
	}
	if rec.PID == os.Getpid() {
		return nil
	}
	if !daemonIsProcessAlive(rec.PID) {
		return removeDaemonLockFile()
	}
	return fmt.Errorf(
		"parallel mode requires isolated --state-dir; existing daemon is active (existing_pid=%d existing_port=%d state_dir=%s)",
		rec.PID,
		rec.Port,
		rec.StateDir,
	)
}

func performDefaultTakeover(server *Server, stateDir string, port int, rec *daemonLockRecord) error {
	if rec.PID <= 0 || rec.Port <= 0 {
		return fmt.Errorf(
			"invalid daemon lock metadata for state_dir=%s (pid=%d, port=%d). remove %s and retry",
			stateDir,
			rec.PID,
			rec.Port,
			daemonLockFilePathForError(),
		)
	}
	if rec.PID == os.Getpid() {
		return nil
	}
	if !daemonIsProcessAlive(rec.PID) {
		return removeDaemonLockFile()
	}

	pidFromPortFile := readPIDFile(rec.Port)
	if pidFromPortFile != rec.PID {
		// Safety guard: if the lock PID mismatches the PID file, never kill blindly.
		// But if the target port is not serving, this is stale lock state and we can reclaim it.
		if !daemonIsServerRunning(rec.Port) {
			server.logLifecycle("daemon_lock_reclaimed_stale_mismatch", port, map[string]any{
				"state_dir":      stateDir,
				"lock_pid":       rec.PID,
				"lock_port":      rec.Port,
				"pid_file":       pidFromPortFile,
				"port_in_use":    false,
				"reclaimed_lock": true,
			})
			removePIDFile(rec.Port)
			return removeDaemonLockFile()
		}
		return fmt.Errorf(
			"daemon ownership mismatch for state_dir=%s: lock pid=%d port=%d, pid_file=%d, port_in_use=true; refusing takeover",
			stateDir,
			rec.PID,
			rec.Port,
			pidFromPortFile,
		)
	}

	server.logLifecycle("daemon_takeover", port, map[string]any{
		"existing_pid":  rec.PID,
		"existing_port": rec.Port,
		"takeover":      true,
		"state_dir":     stateDir,
		"new_pid":       os.Getpid(),
	})

	_ = daemonTryShutdown(rec.Port)
	if !daemonWaitForPortRelease(rec.Port, 2*time.Second) {
		daemonTerminatePID(rec.PID, false)
		if !daemonWaitForPortRelease(rec.Port, 2*time.Second) {
			daemonTerminatePID(rec.PID, true)
			if !daemonWaitForPortRelease(rec.Port, 2*time.Second) {
				return fmt.Errorf(
					"failed to takeover existing daemon for state_dir=%s (existing_pid=%d existing_port=%d)",
					stateDir,
					rec.PID,
					rec.Port,
				)
			}
		}
	}

	removePIDFile(rec.Port)
	return removeDaemonLockFile()
}
