// lifecycle.go — Manages daemon lock ownership, liveness classification, and stale-daemon takeover for singleton enforcement.
// Why: Prevents port conflicts and zombie daemons by coordinating lifecycle via PID-based lock records.

package daemonlife

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// SignalSource describes a process signal for correlated shutdown diagnostics.
func SignalSource(signal os.Signal) string {
	switch signal {
	case os.Interrupt:
		return "Ctrl+C (SIGINT)"
	case syscall.SIGTERM:
		return "SIGTERM (likely --stop or kill)"
	case syscall.SIGHUP:
		return "SIGHUP (terminal closed)"
	default:
		return signal.String()
	}
}

// LaunchOptions describes how this daemon instance was launched.
type LaunchOptions struct {
	Parallel bool
}

// ParseVersionParts splits an optionally v-prefixed semantic version into integers.
func ParseVersionParts(value string) []int {
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return nil
	}
	segments := strings.Split(value, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		part, err := strconv.Atoi(segment)
		if err != nil {
			break
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func sameNonEmptyVersion(a, b string) bool {
	return a != "" && b != "" && !IsNewerVersion(a, b) && !IsNewerVersion(b, a)
}

// IsNewerVersion reports whether candidate is strictly newer than current.
func IsNewerVersion(candidate, current string) bool {
	candidateParts := ParseVersionParts(candidate)
	currentParts := ParseVersionParts(current)
	if candidateParts == nil || currentParts == nil {
		return false
	}
	count := len(candidateParts)
	if len(currentParts) > count {
		count = len(currentParts)
	}
	for index := 0; index < count; index++ {
		candidatePart, currentPart := 0, 0
		if index < len(candidateParts) {
			candidatePart = candidateParts[index]
		}
		if index < len(currentParts) {
			currentPart = currentParts[index]
		}
		if candidatePart != currentPart {
			return candidatePart > currentPart
		}
	}
	return false
}

type daemonLockRecord struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	StateDir  string `json:"state_dir"`
	Version   string `json:"version,omitempty"`
	UpdatedAt string `json:"updated_at"`
	// InstallEpoch is the writer's install epoch (see install_epoch.go). At the same
	// version it is the takeover tiebreaker: a strictly newer epoch supersedes.
	InstallEpoch int64 `json:"install_epoch,omitempty"`
}

// This package's OWN seams (see deps.go for the rule): daemonlife can satisfy
// these itself, so they stay package-local and are swapped by its own tests.
var (
	daemonNow   = time.Now
	daemonSleep = time.Sleep
	// daemonInstallEpoch reports THIS daemon's install epoch (the takeover
	// tiebreaker at equal versions). Injectable for tests.
	daemonInstallEpoch = resolveInstallEpoch
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

// ErrDeferToHealthyDaemon signals that an existing healthy, version-compatible
// daemon is already serving, so this newly-launched daemon must exit cleanly
// (exit 0) instead of taking over. This is what keeps a healthy daemon alive
// when a redundant instance is launched (bridge respawn, second MCP client).
var ErrDeferToHealthyDaemon = errors.New("existing healthy daemon is serving; deferring")

// Deferral carries WHICH daemon is being deferred to.
//
// The lock is per state directory, not per port, so the incumbent may be
// serving a different port than the one this instance was asked to bind. The
// old message interpolated the REQUESTED port and so announced "a healthy
// daemon is already serving on port 7890" while the healthy daemon was on
// 19310 and nothing listened on 7890 at all — which is precisely the case an
// operator most needs named accurately.
type Deferral struct {
	PID     int
	Port    int
	Version string
}

func (d *Deferral) Error() string {
	return fmt.Sprintf("existing healthy daemon (pid=%d, port=%d, version=%s) is serving; deferring", d.PID, d.Port, d.Version)
}

// Unwrap keeps errors.Is(err, ErrDeferToHealthyDaemon) working for callers that
// only need to know they must exit cleanly.
func (d *Deferral) Unwrap() error { return ErrDeferToHealthyDaemon }

// daemonProbeHealth reports whether the daemon on port answers /health, its
// reported version, and whether the failure was connection-refused (nothing
// listening — definitively gone, so retrying is pointless). Never blocks past
// the timeout. The transport itself is a host seam (Deps.FetchHealth); the
// timeout policy is ours.
func daemonProbeHealth(d Deps, port int) (reachable bool, version string, refused bool) {
	ctx, cancel := context.WithTimeout(context.Background(), daemonHealthProbeTimeout)
	defer cancel()
	return d.FetchHealth(ctx, port, daemonHealthProbeTimeout)
}

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
// daemon described by rec (returns ErrDeferToHealthyDaemon) or TAKE OVER from it
// (returns nil). Invariant: a HEALTHY daemon whose version is >= ours is never
// killed. It is taken over only when it is STALLED (no /health after retries) or
// strictly OLDER than us (a real upgrade). A daemon still inside the startup
// grace window is always deferred to.
func classifyExistingDaemon(d Deps, port int, rec *daemonLockRecord) error {
	// A strict version upgrade always replaces the incumbent — even one still
	// inside the startup grace window — because running the newer binary is the
	// whole point of the install. Uses the registered version so we needn't wait
	// on a /health round-trip for the common upgrade case.
	if rec.Version != "" && IsNewerVersion(d.Version, rec.Version) {
		d.Log.LogLifecycle("daemon_takeover_upgrade", port, map[string]any{
			"existing_pid":     rec.PID,
			"existing_version": rec.Version,
			"our_version":      d.Version,
		})
		return nil
	}

	// Same version, but a strictly NEWER install supersedes an older one — the
	// "latest install always wins" tiebreaker (install_epoch.go). Without it, two
	// same-version installs (e.g. ~/.kaboom/bin vs an npm-global copy) have no way to
	// pick a winner and thrash. Uses the registered epoch (no /health round-trip),
	// and fires before the startup-grace defer so a fresh install takes over at once.
	// An equal-or-older epoch never reaches the takeover here, so this cannot
	// ping-pong: the older install always defers to the newer one.
	if sameNonEmptyVersion(d.Version, rec.Version) {
		if ourEpoch := daemonInstallEpoch(d.Recovery); ourEpoch > 0 && ourEpoch > rec.InstallEpoch {
			d.Log.LogLifecycle("daemon_takeover_newer_install", port, map[string]any{
				"existing_pid":           rec.PID,
				"existing_install_epoch": rec.InstallEpoch,
				"our_install_epoch":      ourEpoch,
				"version":                d.Version,
			})
			return nil
		}
	}

	// Not a known upgrade. Never kill a daemon that only just registered — it may
	// still be binding its ports, and killing it is how two near-simultaneous
	// launches ping-pong (A kills B, B's respawn kills A). A negative age (lock
	// timestamped in the near future, e.g. a backward clock/NTP step during the
	// grace window) is by definition brand-new, so it must defer too — the old
	// `age >= 0` guard let clock skew skip grace and take over a healthy young
	// daemon. A far-future/corrupt timestamp erring toward defer is the safe choice
	// (never kill on bad data).
	if age, ok := daemonLockAge(rec); ok && age < daemonStartupGrace {
		d.Log.LogLifecycle("daemon_defer_starting", port, map[string]any{
			"existing_pid":  rec.PID,
			"existing_port": rec.Port,
			"age_ms":        age.Milliseconds(),
		})
		return &Deferral{PID: rec.PID, Port: rec.Port, Version: rec.Version}
	}

	// Probe liveness, retried across the window so a momentarily busy-but-healthy
	// daemon answers before we ever conclude it is stalled. A connection-refused
	// probe means nothing is accepting on the port right now — but that is only
	// DEFINITIVE ("gone") when the incumbent PID is ALSO dead, in which case we break
	// immediately instead of burning the retry budget's sleeps on a certainty
	// (finding L's fast path). A refused probe while the PID is still ALIVE is NOT
	// proof of death: a healthy daemon can transiently refuse (listen backlog full)
	// or be mid graceful-shutdown, so we retry rather than SIGTERM a live, healthy
	// peer on a single stray refusal (which would feed the takeover war). A
	// persistently-wedged live daemon is still reclaimed once the retry budget is
	// exhausted below.
	reachable := false
	liveVersion := ""
	for attempt := 0; attempt < daemonHealthProbeRetries; attempt++ {
		var refused bool
		if reachable, liveVersion, refused = daemonProbeHealth(d, rec.Port); reachable {
			break
		}
		if refused && !d.IsProcessAlive(rec.PID) {
			break
		}
		if attempt < daemonHealthProbeRetries-1 {
			daemonSleep(daemonHealthProbeBackoff)
		}
	}

	if !reachable {
		// Stalled: alive PID, owns the port, but never answered /health. Replace it.
		d.Log.LogLifecycle("daemon_takeover_stalled", port, map[string]any{
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
	if existingVersion != "" && IsNewerVersion(d.Version, existingVersion) {
		d.Log.LogLifecycle("daemon_takeover_upgrade", port, map[string]any{
			"existing_pid":     rec.PID,
			"existing_version": existingVersion,
			"our_version":      d.Version,
		})
		return nil
	}

	d.Log.LogLifecycle("daemon_defer_healthy", port, map[string]any{
		"existing_pid":     rec.PID,
		"existing_port":    rec.Port,
		"existing_version": existingVersion,
		"our_version":      d.Version,
	})
	return &Deferral{PID: rec.PID, Port: rec.Port, Version: existingVersion}
}

// EnforceStartupPolicy is the single-instance gate a starting daemon must pass.
// It returns nil when this daemon may proceed, ErrDeferToHealthyDaemon when a
// healthy incumbent should keep serving (the caller must exit 0), or an error
// describing why startup cannot safely continue.
func EnforceStartupPolicy(d Deps, port int, opts LaunchOptions) error {
	stateDir, err := state.RootDir()
	if err != nil {
		return fmt.Errorf("cannot resolve state dir: %w", err)
	}
	rec, err := readDaemonLockFile()
	if err != nil {
		d.Log.LogLifecycle("daemon_lock_recovered", port, map[string]any{"reason": "state_read_or_parse_failed"})
		if d.Recovery != nil {
			d.Recovery.Report(statediag.Diagnostic{
				Name:   "daemon_lock_state",
				Detail: "Daemon ownership state was malformed; the stale lock was removed and startup continued.",
				Fix:    "If this recurs, check permissions and disk health for the Kaboom run-state directory.",
			})
		}
		if removeErr := removeDaemonLockFile(); removeErr != nil {
			return fmt.Errorf("cannot recover malformed daemon lock: %w", removeErr)
		}
		return nil
	}
	if rec == nil {
		return nil
	}

	if opts.Parallel {
		return validateParallelIsolation(d, rec)
	}
	return performDefaultTakeover(d, stateDir, port, rec)
}

func validateParallelIsolation(d Deps, rec *daemonLockRecord) error {
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
	if !d.IsProcessAlive(rec.PID) {
		return removeDaemonLockFile()
	}
	return fmt.Errorf(
		"parallel mode requires isolated --state-dir; existing daemon is active (existing_pid=%d existing_port=%d state_dir=%s)",
		rec.PID,
		rec.Port,
		rec.StateDir,
	)
}

func performDefaultTakeover(d Deps, stateDir string, port int, rec *daemonLockRecord) error {
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
	if !d.IsProcessAlive(rec.PID) {
		return removeDaemonLockFile()
	}

	pidFromPortFile := d.ReadPIDFile(rec.Port)
	if pidFromPortFile != rec.PID {
		// Safety guard: if the lock PID mismatches the PID file, never kill blindly.
		// But if the target port is not serving, this is stale lock state and we can reclaim it.
		if !d.IsServerRunning(rec.Port) {
			d.Log.LogLifecycle("daemon_lock_reclaimed_stale_mismatch", port, map[string]any{
				"state_dir":      stateDir,
				"lock_pid":       rec.PID,
				"lock_port":      rec.Port,
				"pid_file":       pidFromPortFile,
				"port_in_use":    false,
				"reclaimed_lock": true,
			})
			d.RemovePIDFile(rec.Port)
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
	if decision := classifyExistingDaemon(d, port, rec); decision != nil {
		return decision
	}

	d.Log.LogLifecycle("daemon_takeover", port, map[string]any{
		"existing_pid":  rec.PID,
		"existing_port": rec.Port,
		"takeover":      true,
		"state_dir":     stateDir,
		"new_pid":       os.Getpid(),
	})

	_ = d.TryShutdown(rec.Port)
	if !d.WaitForPortRelease(rec.Port, 2*time.Second) {
		d.TerminatePID(rec.PID, false)
		if !d.WaitForPortRelease(rec.Port, 2*time.Second) {
			d.TerminatePID(rec.PID, true)
			if !d.WaitForPortRelease(rec.Port, 2*time.Second) {
				return fmt.Errorf(
					"failed to takeover existing daemon for state_dir=%s (existing_pid=%d existing_port=%d)",
					stateDir,
					rec.PID,
					rec.Port,
				)
			}
		}
	}

	d.RemovePIDFile(rec.Port)
	return removeDaemonLockFile()
}
