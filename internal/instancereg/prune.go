// prune.go — Removes registry entries whose owner is gone, and reports owners
// that are alive but no longer heartbeating.
// Why: liveness must be identity-checked, never pid-only. On one developer machine
// lock records written 8 Aug named pids 4240/4411/4642, which the OS had since
// reassigned to TextInputSwitcher and two Adobe Creative Cloud helpers; a
// kill(pid,0) check reported all three alive, so those entries could never expire.

package instancereg

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procidentity"
)

// Prune deletes records whose owning process is no longer running AS the process
// that wrote them. It returns how many were removed.
//
// A record whose owner is alive is NEVER removed, even when its heartbeat is
// stale: that process still holds its ports, so forgetting it would let a
// replacement start and collide. Stale-but-alive is reported by Wedged instead.
func Prune(now time.Time, _ time.Duration) (int, error) {
	records, err := List()
	if err != nil {
		return 0, err
	}
	snapshot, err := procidentity.Snapshot()
	if err != nil {
		// Without a process listing we cannot prove anything is dead. Removing
		// records here would be a guess that silently uncaps the machine.
		return 0, fmt.Errorf("instancereg: cannot take process snapshot: %w", err)
	}
	removed := 0
	for _, rec := range records {
		if procidentity.AliveIn(snapshot, rec.PID, rec.Identity) {
			continue
		}
		if err := os.Remove(rec.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("instancereg: remove stale record for pid %d: %w", rec.PID, err)
		}
		removed++
	}
	return removed, nil
}

// Live returns the records whose owning process is still running as itself.
func Live() ([]Record, error) {
	records, err := List()
	if err != nil {
		return nil, err
	}
	snapshot, err := procidentity.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("instancereg: cannot take process snapshot: %w", err)
	}
	live := make([]Record, 0, len(records))
	for _, rec := range records {
		if procidentity.AliveIn(snapshot, rec.PID, rec.Identity) {
			live = append(live, rec)
		}
	}
	return live, nil
}
