// startup_throttle.go — self-defense against crash-loop restart storms.
// If the SAME install (version + install epoch) restarts too many times within a
// short window, the daemon logs LOUDLY and waits a bounded delay before binding, so
// a pathological loop degrades gracefully instead of hammering launchd (which throttles
// or disables the LaunchAgent, taking the whole service — including the terminal
// server on port+1 — dark). It NEVER refuses to start: it only warns and delays.
//
// A legitimate upgrade or install-epoch takeover is NOT a crash-restart: those change
// the (version, epoch) identity, which resets the counter so a real deploy is never
// penalized.

package main

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
// keyed by (Version, InstallEpoch) so a different install (upgrade / epoch takeover)
// starts from a clean slate rather than inheriting the previous install's restarts.
type restartHistory struct {
	Version      string  `json:"version"`
	InstallEpoch int64   `json:"install_epoch"`
	Timestamps   []int64 `json:"restart_unixnano"`
}

// daemonThrottleSleep is the injectable sleep used by the startup restart throttle.
var daemonThrottleSleep = time.Sleep

// recordRestartAndComputeDelay appends `now` to the restart history for (ver, epoch)
// and returns the updated history plus the bounded startup delay to apply. It never
// signals refusal — only a delay (0 when under the threshold). A history whose
// identity differs from (ver, epoch) is reset first: an upgrade or epoch takeover is
// not a crash-restart. Timestamps older than the window are pruned.
func recordRestartAndComputeDelay(prev restartHistory, now time.Time, ver string, epoch int64) (restartHistory, time.Duration) {
	if prev.Version != ver || prev.InstallEpoch != epoch {
		prev = restartHistory{Version: ver, InstallEpoch: epoch}
	}
	cutoff := now.Add(-restartThrottleWindow)
	kept := make([]int64, 0, len(prev.Timestamps)+1)
	for _, ts := range prev.Timestamps {
		if time.Unix(0, ts).After(cutoff) {
			kept = append(kept, ts)
		}
	}
	kept = append(kept, now.UnixNano())
	next := restartHistory{Version: ver, InstallEpoch: epoch, Timestamps: kept}

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

// applyStartupRestartThrottle records this daemon start and, if the same install has
// restarted too many times too fast, logs loudly and waits a bounded delay before
// returning. It NEVER refuses to start. Returns the delay applied (0 if none). Any
// state-dir / IO problem is non-fatal (best effort: startup proceeds without a delay).
func applyStartupRestartThrottle(server *Server, port int) time.Duration {
	stateDir, err := state.RootDir()
	if err != nil {
		server.logLifecycle("restart_history_statedir_failed", port, map[string]any{"error": err.Error()})
		return 0
	}
	path := restartHistoryPath(stateDir)
	prev := loadRestartHistory(path)
	next, delay := recordRestartAndComputeDelay(prev, daemonNow(), version, daemonInstallEpoch())
	if err := saveRestartHistory(path, next); err != nil {
		server.logLifecycle("restart_history_write_failed", port, map[string]any{"error": err.Error()})
	}
	if delay <= 0 {
		return 0
	}
	server.logLifecycle("restart_storm_throttle", port, map[string]any{
		"restarts_in_window": len(next.Timestamps),
		"window_seconds":     int(restartThrottleWindow.Seconds()),
		"delay_ms":           delay.Milliseconds(),
		"version":            version,
		"install_epoch":      next.InstallEpoch,
	})
	stderrf("[Kaboom] WARNING: this install restarted %d times within %s — possible crash loop. "+
		"Backing off %s before binding to avoid launchd throttling.\n",
		len(next.Timestamps), restartThrottleWindow, delay)
	daemonThrottleSleep(delay)
	return delay
}
