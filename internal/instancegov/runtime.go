// runtime.go — Keeps an admitted instance's registry entry fresh.
// Why: a heartbeat is what separates "running" from "wedged". Every admitted
// instance must run this, so it lives here rather than being re-implemented (and
// eventually forgotten) at each call site.

package instancegov

import (
	"context"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/idlewatch"
)

// DefaultHeartbeatInterval is how often an admitted instance republishes its
// liveness. The reaper's TTL must be a multiple of this so a single slow write
// never makes a healthy instance look wedged.
const DefaultHeartbeatInterval = 10 * time.Second

// DefaultHeartbeatTTL is how stale a heartbeat may get before the reaper treats
// the instance as wedged. Three missed beats, not one.
const DefaultHeartbeatTTL = 3 * DefaultHeartbeatInterval

// StartHeartbeat republishes liveness until ctx is cancelled. A failed write is
// reported through onError rather than swallowed: silent heartbeat failure would
// make a healthy daemon look wedged and get it killed.
func (r *Result) StartHeartbeat(ctx context.Context, interval time.Duration, onError func(error)) {
	if r == nil || r.Handle == nil {
		return
	}
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.Heartbeat(); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
}

// StartIdleWatch shuts the instance down once it has no work left, or once a test
// instance outlives its run. onExit is called at most once with the reason.
func (r *Result) StartIdleWatch(ctx context.Context, cfg idlewatch.Config, onExit func(string)) {
	if r == nil {
		return
	}
	watcher := idlewatch.New(cfg, time.Now())
	go watcher.Run(ctx, onExit)
}
