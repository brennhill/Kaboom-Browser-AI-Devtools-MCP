// Purpose: Manages daemon lock file, process liveness checks, and stale-daemon cleanup for singleton enforcement.
// Why: Prevents port conflicts and zombie daemons by coordinating lifecycle via PID-based lock records.

package main

import (
	"context"
	"errors"
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
	daemonSleep              = time.Sleep

	// daemonProbeHealth reports whether the daemon on port answers /health, and
	// its reported version. Injectable for tests. Never blocks past the timeout.
	daemonProbeHealth = func(port int) (reachable bool, version string) {
		ctx, cancel := context.WithTimeout(context.Background(), daemonHealthProbeTimeout)
		defer cancel()
		h := fetchInstallHealth(ctx, port, daemonHealthProbeTimeout)
		return h.reachable, h.version
	}
)

const (
	// A daemon that registered its lock this recently is still coming up; never
	// kill it, or two racing launches ping-pong (A kills B, B's respawn kills A).
	daemonStartupGrace = 5 * time.Second
	// Per-probe timeout and retry budget. Retrying across ~1.5s lets a momentarily
	// busy-but-healthy daemon answer before we ever conclude it is stalled.
	daemonHealthProbeTimeout = 800 * time.Millisecond
	daemonHealthProbeRetries = 3
	daemonHealthProbeBackoff = 500 * time.Millisecond
)

// errDeferToHealthyDaemon signals that an existing healthy, version-compatible
// daemon is already serving, so this newly-launched daemon must exit cleanly
// (exit 0) instead of taking over. This is what keeps a healthy daemon alive
// when a redundant instance is launched (bridge respawn, second MCP client).
var errDeferToHealthyDaemon = errors.New("existing healthy daemon is serving; deferring")

// daemonLockAge returns how long ago the lock was written (its registration
// time), and whether that could be determined.
func daemonLockAge(rec *daemonLockRecord) (time.Duration, bool) {
	if rec == nil || rec.UpdatedAt == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, rec.UpdatedAt)
	if err != nil {
		return 0, false
	}
	return daemonNow().Sub(t), true
}

// classifyExistingDaemon decides whether a starting daemon should DEFER to the
// daemon described by rec (returns errDeferToHealthyDaemon) or TAKE OVER from it
// (returns nil). Invariant: a HEALTHY daemon whose version is >= ours is never
// killed. It is taken over only when it is STALLED (no /health after retries) or
// strictly OLDER than us (a real upgrade). A daemon still inside the startup
// grace window is always deferred to.
func classifyExistingDaemon(server *Server, port int, rec *daemonLockRecord) error {
	// A strict version upgrade always replaces the incumbent — even one still
	// inside the startup grace window — because running the newer binary is the
	// whole point of the install. Uses the registered version so we needn't wait
	// on a /health round-trip for the common upgrade case.
	if rec.Version != "" && isNewerVersion(version, rec.Version) {
		server.logLifecycle("daemon_takeover_upgrade", port, map[string]any{
			"existing_pid":     rec.PID,
			"existing_version": rec.Version,
			"our_version":      version,
		})
		return nil
	}

	// Not a known upgrade. Never kill a daemon that only just registered — it may
	// still be binding its ports, and killing it is how two near-simultaneous
	// launches ping-pong (A kills B, B's respawn kills A).
	if age, ok := daemonLockAge(rec); ok && age >= 0 && age < daemonStartupGrace {
		server.logLifecycle("daemon_defer_starting", port, map[string]any{
			"existing_pid":  rec.PID,
			"existing_port": rec.Port,
			"age_ms":        age.Milliseconds(),
		})
		return errDeferToHealthyDaemon
	}

	// Probe liveness, retried across the window so a momentarily busy-but-healthy
	// daemon answers before we ever conclude it is stalled.
	reachable := false
	liveVersion := ""
	for attempt := 0; attempt < daemonHealthProbeRetries; attempt++ {
		if reachable, liveVersion = daemonProbeHealth(rec.Port); reachable {
			break
		}
		if attempt < daemonHealthProbeRetries-1 {
			daemonSleep(daemonHealthProbeBackoff)
		}
	}

	if !reachable {
		// Stalled: alive PID, owns the port, but never answered /health. Replace it.
		server.logLifecycle("daemon_takeover_stalled", port, map[string]any{
			"existing_pid":  rec.PID,
			"existing_port": rec.Port,
		})
		return nil
	}

	existingVersion := liveVersion
	if existingVersion == "" {
		existingVersion = rec.Version
	}

	// Healthy. If the live version turns out older than us (registered version was
	// blank/stale), treat it as an upgrade; otherwise a healthy same-or-newer
	// daemon is never killed — defer and exit cleanly.
	if existingVersion != "" && isNewerVersion(version, existingVersion) {
		server.logLifecycle("daemon_takeover_upgrade", port, map[string]any{
			"existing_pid":     rec.PID,
			"existing_version": existingVersion,
			"our_version":      version,
		})
		return nil
	}

	server.logLifecycle("daemon_defer_healthy", port, map[string]any{
		"existing_pid":     rec.PID,
		"existing_version": existingVersion,
		"our_version":      version,
	})
	return errDeferToHealthyDaemon
}

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

	// The existing daemon is alive and owns its port. Never kill it blindly:
	// defer to a healthy, version-compatible one; take over only a stalled or
	// older instance. This is the guard against killing a healthy daemon and
	// against launch-vs-launch restart storms.
	if decision := classifyExistingDaemon(server, port, rec); decision != nil {
		return decision
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
