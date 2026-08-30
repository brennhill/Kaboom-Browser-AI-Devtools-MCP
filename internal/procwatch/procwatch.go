// procwatch.go — Notices when the process that started us has gone away.
// Why: a bridge exists to serve exactly ONE MCP client over stdio, so when that
// client dies the bridge has no remaining purpose — yet nothing told it so. Bridges
// outnumber daemons roughly 11:1 in this project's own lifecycle logs, and two were
// found alive after 31 hours holding ~24MB each. stdin EOF covers the clean case;
// this covers the client that was SIGKILLed and never closed the pipe.
// Docs: docs/core/reliability/zombie-prevention.md

package procwatch

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Config describes which parent to watch and how often.
type Config struct {
	// OriginalPPID is the parent observed at startup. A value of 1 or less means
	// the process was already orphaned (a deliberately daemonized process), and
	// watching is disabled — otherwise a daemon would exit the moment it started.
	OriginalPPID int
	// Poll is the check interval. Defaults to 2s: this only needs to reclaim a
	// process within a human-noticeable window, not instantly.
	Poll time.Duration
	// CurrentPPID reads the live parent pid. Defaults to os.Getppid.
	CurrentPPID func() int
}

// ParentGone reports whether the original parent has exited. Any change of parent
// means the original is gone: on macOS an orphan is adopted by launchd (pid 1),
// and on Linux by pid 1 or by the nearest subreaper, so comparing against the
// ORIGINAL covers both rather than only testing for pid 1.
func ParentGone(originalPPID, currentPPID int) bool {
	if originalPPID <= 1 {
		return false
	}
	return currentPPID != originalPPID
}

// Watch polls until the parent disappears or ctx is cancelled, then calls onGone
// exactly once. It returns without calling onGone when the context is cancelled or
// when the parent is unwatchable.
func Watch(ctx context.Context, cfg Config, onGone func(reason string)) {
	if cfg.Poll <= 0 {
		cfg.Poll = 2 * time.Second
	}
	if cfg.CurrentPPID == nil {
		cfg.CurrentPPID = os.Getppid
	}
	if cfg.OriginalPPID <= 1 {
		// EXPECTED_ABSENCE: a daemonized process is orphaned by design. There is
		// no parent to outlive, so watching would be a guaranteed false positive.
		return
	}
	ticker := time.NewTicker(cfg.Poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := cfg.CurrentPPID()
			if ParentGone(cfg.OriginalPPID, current) {
				if onGone != nil {
					onGone(fmt.Sprintf(
						"parent process %d exited (reparented to %d); this process has no remaining client",
						cfg.OriginalPPID, current))
				}
				return
			}
		}
	}
}
