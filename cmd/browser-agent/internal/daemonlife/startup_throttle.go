// startup_throttle.go — self-defense against crash-loop restart storms.
// If the SAME daemon instance (version + install epoch + port) restarts too many
// times within a short window, the daemon logs LOUDLY and waits a bounded delay before binding, so
// a pathological loop degrades gracefully instead of hammering launchd (which throttles
// or disables the LaunchAgent, taking the whole service — including the terminal
// server on port+1 — dark). It NEVER refuses to start: it only warns and delays.
//
// A legitimate upgrade or install-epoch takeover is NOT a crash-restart: those change
// the identity, which resets the counter so a real deploy is never penalized. Port is
// part of that identity too — see restartHistory.

package daemonlife

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

const (
	// restartThrottleWindow is the sliding window over which restarts are counted.
	restartThrottleWindow = 30 * time.Second
	// restartThrottleMax is the number of restarts allowed in the window before the
	// backoff engages. More than this many (i.e. the N+1-th) starts delaying.
	restartThrottleMax = 5
	// restartThrottleStep is the per-excess-restart delay increment.
	restartThrottleStep = 500 * time.Millisecond
	// restartThrottleCap bounds the delay so we never stall startup for long.
	restartThrottleCap = 3 * time.Second

	// restartHistoryFileName is the state-dir file tracking recent restart times.
	restartHistoryFileName = "restart-history.json"
)

// restartHistory is the persisted record of this install's recent restarts. It is
// keyed by (Version, InstallEpoch, Port) so a different install (upgrade / epoch
// takeover) — or a different daemon instance — starts from a clean slate rather than
// inheriting unrelated restarts.
//
// Port is part of the identity because a restart storm is about ONE endpoint being
// restarted repeatedly, which is what launchd throttles. Without it, every daemon
// sharing a state dir was counted together: the Go test suite spawns many short-lived
// daemons on distinct random ports in one state dir, crossed the threshold, and then
// throttled every subsequent start into "Server failed to start". Production always
// binds the same port, so a genuine crash loop still accumulates.
type restartHistory struct {
	Version      string  `json:"version"`
	InstallEpoch int64   `json:"install_epoch"`
	Port         int     `json:"port"`
	Timestamps   []int64 `json:"restart_unixnano"`
}

// daemonThrottleSleep is the injectable sleep used by the startup restart throttle.
var daemonThrottleSleep = time.Sleep

// recordRestartAndComputeDelay appends `now` to the restart history for
// (ver, epoch, port) and returns the updated history plus the bounded startup delay
// to apply. It never signals refusal — only a delay (0 when under the threshold). A
// history whose identity differs is reset first: an upgrade, an epoch takeover, or a
// different port is not a restart of this instance. Older timestamps are pruned.
func recordRestartAndComputeDelay(prev restartHistory, now time.Time, ver string, epoch int64, port int) (restartHistory, time.Duration) {
	if prev.Version != ver || prev.InstallEpoch != epoch || prev.Port != port {
		prev = restartHistory{Version: ver, InstallEpoch: epoch, Port: port}
	}
	cutoff := now.Add(-restartThrottleWindow)
	kept := make([]int64, 0, len(prev.Timestamps)+1)
	for _, ts := range prev.Timestamps {
		if time.Unix(0, ts).After(cutoff) {
			kept = append(kept, ts)
		}
	}
	kept = append(kept, now.UnixNano())
	next := restartHistory{Version: ver, InstallEpoch: epoch, Port: port, Timestamps: kept}

	// Under the threshold: no delay. Only the (max+1)-th and beyond within the window
	// back off, escalating one step per excess restart, capped so startup never stalls.
	if len(kept) <= restartThrottleMax {
		return next, 0
	}
	over := len(kept) - restartThrottleMax // 1, 2, 3, ...
	delay := time.Duration(over) * restartThrottleStep
	if delay > restartThrottleCap {
		delay = restartThrottleCap
	}
	return next, delay
}

// restartHistoryPath returns the restart-history file path under the given state dir.
func restartHistoryPath(stateDir string) string {
	return filepath.Join(stateDir, restartHistoryFileName)
}

// loadRestartHistory reads the restart history; a missing or corrupt file yields an
// empty history (treated as a fresh install), never an error.
func loadRestartHistory(path string) restartHistory {
	b, err := os.ReadFile(path) // #nosec G304 -- path derived from our own state dir
	if err != nil {
		return restartHistory{}
	}
	var h restartHistory
	if err := json.Unmarshal(b, &h); err != nil {
		return restartHistory{}
	}
	return h
}

// saveRestartHistory persists the restart history, creating the state dir if needed.
func saveRestartHistory(path string, h restartHistory) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ClearRestartHistoryOnCleanShutdown removes the restart history after a graceful,
// signal-initiated shutdown. A clean shutdown means we are NOT crash-looping, so the
// restart counter resets — this is what keeps legitimate stop/start cycles (a user
// restart, or a test suite's rapid --stop/spawn) from ever engaging the backoff. A
// crash (uncaught SIGPIPE, panic, or an unexpected http_listener_died) never runs this
// path, so its restart still counts toward the storm threshold. Best effort: a failure
// to remove the file is logged, never fatal.
func ClearRestartHistoryOnCleanShutdown(d Deps, port int) {
	stateDir, err := state.RootDir()
	if err != nil {
		return
	}
	path := restartHistoryPath(stateDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		d.Log.LogLifecycle("restart_history_clear_failed", port, map[string]any{"error": err.Error()})
	}
}

// ApplyStartupRestartThrottle records this daemon start and, if the same install has
// restarted too many times too fast, logs loudly and waits a bounded delay before
// returning. It NEVER refuses to start. Returns the delay applied (0 if none). Any
// state-dir / IO problem is non-fatal (best effort: startup proceeds without a delay).
func ApplyStartupRestartThrottle(d Deps, port int) time.Duration {
	stateDir, err := state.RootDir()
	if err != nil {
		d.Log.LogLifecycle("restart_history_statedir_failed", port, map[string]any{"error": err.Error()})
		return 0
	}
	path := restartHistoryPath(stateDir)
	prev := loadRestartHistory(path)
	next, delay := recordRestartAndComputeDelay(prev, daemonNow(), d.Version, daemonInstallEpoch(), port)
	if err := saveRestartHistory(path, next); err != nil {
		d.Log.LogLifecycle("restart_history_write_failed", port, map[string]any{"error": err.Error()})
	}
	if delay <= 0 {
		return 0
	}
	d.Log.LogLifecycle("restart_storm_throttle", port, map[string]any{
		"restarts_in_window": len(next.Timestamps),
		"window_seconds":     int(restartThrottleWindow.Seconds()),
		"delay_ms":           delay.Milliseconds(),
		"version":            d.Version,
		"install_epoch":      next.InstallEpoch,
	})
	d.Warnf("[Kaboom] WARNING: this install restarted %d times within %s — possible crash loop. "+
		"Backing off %s before binding to avoid launchd throttling.\n",
		len(next.Timestamps), restartThrottleWindow, delay)
	daemonThrottleSleep(delay)
	return delay
}
