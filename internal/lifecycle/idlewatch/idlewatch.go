// idlewatch.go — Bounds a daemon's life by usefulness rather than by nothing.
// Why: the daemon was documented to "persist until explicitly stopped", so any
// daemon a developer or a crashed test run forgot about held two ports and ~37MB
// indefinitely — one on this machine had been up 2 days 13 hours serving nobody.
// A daemon that no one is using should release its resources; a daemon someone IS
// using must never be shut down under them, which is why every unknown answers
// "busy".
// Docs: docs/core/reliability/zombie-prevention.md

package idlewatch

import (
	"context"
	"fmt"
	"time"
)

// BusyProbe reports whether the daemon currently has work that must not be
// interrupted, plus a human-readable reason for diagnostics. Implementations must
// count connected MCP clients, an attached browser extension, active recordings,
// live PTY sessions, and in-flight requests.
type BusyProbe func() (busy bool, reason string)

// Config is the lifetime budget for one daemon.
type Config struct {
	// IdleAfter is how long the daemon may sit with no work before exiting.
	// Zero disables idle exit.
	IdleAfter time.Duration
	// MaxLifetime is a hard bound applied even while busy. It exists for
	// --parallel daemons, which belong to a single test run and must not outlive
	// it when that run dies without stopping them. Zero means unlimited.
	MaxLifetime time.Duration
	// Poll is how often Run evaluates. Defaults to 30s.
	Poll time.Duration
	// Busy probes for work in progress. A nil probe means "cannot tell", which is
	// always treated as busy: exiting on an unknown state would kill a daemon
	// mid-recording.
	Busy BusyProbe
}

// Watcher tracks how long a daemon has been idle. It holds no timers of its own so
// that every decision is a pure function of the clock value passed to Tick, which
// is what makes its tests deterministic instead of sleep-based.
type Watcher struct {
	cfg       Config
	startedAt time.Time
	idleSince time.Time
	idle      bool
}

// New creates a Watcher whose lifetime is measured from startedAt.
//
// The watcher starts IDLE from startedAt, not from its first poll: a daemon that
// has never served anyone has been idle since it booted, and starting the window
// at the first tick instead would give every abandoned daemon a free extra window.
func New(cfg Config, startedAt time.Time) *Watcher {
	if cfg.Poll <= 0 {
		cfg.Poll = 30 * time.Second
	}
	return &Watcher{cfg: cfg, startedAt: startedAt, idle: true, idleSince: startedAt}
}

// Tick evaluates one poll at now and reports whether the daemon should shut down.
//
// The idle window is measured from when work last STOPPED, not from start, so any
// activity resets it. The lifetime bound is measured from start and ignores work,
// because its whole purpose is to reclaim a daemon whose owning run has died.
func (w *Watcher) Tick(now time.Time) (bool, string) {
	if w.cfg.MaxLifetime > 0 && now.Sub(w.startedAt) > w.cfg.MaxLifetime {
		return true, fmt.Sprintf("maximum lifetime %s exceeded (uptime %s)",
			w.cfg.MaxLifetime, now.Sub(w.startedAt).Round(time.Second))
	}

	busy, reason := w.probe()
	if busy {
		w.idle = false
		return false, reason
	}

	if !w.idle {
		w.idle = true
		w.idleSince = now
		return false, "idle window started"
	}

	if w.cfg.IdleAfter <= 0 {
		return false, "idle exit disabled"
	}
	if idleFor := now.Sub(w.idleSince); idleFor > w.cfg.IdleAfter {
		return true, fmt.Sprintf("idle for %s with no connected clients", idleFor.Round(time.Second))
	}
	return false, "idle but within the window"
}

// probe answers the busy question, defaulting to busy whenever it cannot be
// answered. See Config.Busy.
func (w *Watcher) probe() (bool, string) {
	if w.cfg.Busy == nil {
		return true, "no activity probe configured; assuming work in progress"
	}
	return w.cfg.Busy()
}

// Run polls until ctx is cancelled or the daemon should exit, then calls onExit
// exactly once with the reason. It never calls onExit for a cancelled context:
// that path is an ordinary shutdown the caller is already handling.
func (w *Watcher) Run(ctx context.Context, onExit func(reason string)) {
	ticker := time.NewTicker(w.cfg.Poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if exit, reason := w.Tick(time.Now()); exit {
				if onExit != nil {
					onExit(reason)
				}
				return
			}
		}
	}
}
