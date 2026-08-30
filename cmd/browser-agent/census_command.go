// census_command.go — The `--instances` and `--reap` operator surface.
// Why: `--force` was the only machine-wide tool and it was a blunt
// `pkill -f "kaboom.*--daemon"` that killed healthy daemons along with leaked ones,
// invoked only during install. These two commands let an operator SEE the machine
// before changing it, and reclaim only what is provably reclaimable.
// Docs: docs/core/reliability/zombie-prevention.md

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reaper"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// staleParallelDirMaxAge is how long an unclaimed generated run directory may
// survive. A day is long enough that an overnight suite is never swept mid-run.
const staleParallelDirMaxAge = 24 * time.Hour

// runCensus prints every registered instance. It prunes dead entries first so the
// listing reflects the machine as it is now, not as it was when something crashed.
func runCensus() int {
	if _, err := instancereg.Prune(time.Now(), instancegov.DefaultHeartbeatTTL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not prune stale entries: %v\n", err)
	}
	records, err := instancereg.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read the instance registry: %v\n", err)
		return 1
	}
	diag.Print(reaper.FormatCensus(records, time.Now()))
	return 0
}

// runReap reclaims dead entries, wedged processes, over-cap instances, and
// abandoned parallel state directories. It never terminates a healthy in-cap
// daemon; see reaper.Plan.
func runReap(dryRun bool) int {
	now := time.Now()
	all, err := instancereg.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read the instance registry: %v\n", err)
		return 1
	}
	live, err := instancereg.Live()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot determine which instances are alive: %v\n", err)
		return 1
	}

	plan := reaper.Plan(reaper.Input{
		Live: live, All: all, Policy: instancegov.DefaultPolicy(),
		HeartbeatTTL: instancegov.DefaultHeartbeatTTL, Now: now,
	})
	for _, action := range plan.Actions {
		diag.Printf("%-5s pid=%-7d role=%-8s ports=%v  %s\n",
			action.Kind, action.Record.PID, action.Record.Role, action.Record.Ports, action.Reason)
	}

	result, applyErr := reaper.Apply(plan, reaper.Deps{
		DryRun:    dryRun,
		Terminate: procctl.TerminatePID,
		Remove:    os.Remove,
	})
	if applyErr != nil {
		fmt.Fprintf(os.Stderr, "some instances could not be reclaimed: %v\n", applyErr)
	}

	sweep := sweepAbandonedParallelDirs(live, now, dryRun)

	verb := "reclaimed"
	if dryRun {
		verb = "would reclaim"
	}
	diag.Printf("\n%s: %d killed, %d stale record(s) pruned, %d abandoned state director(ies).\n",
		verb, result.Killed, result.Pruned, sweep)
	for _, kept := range plan.Keep {
		diag.Printf("keeping pid=%d role=%s ports=%v\n", kept.PID, kept.Role, kept.Ports)
	}
	if applyErr != nil {
		return 1
	}
	return 0
}

func sweepAbandonedParallelDirs(live []instancereg.Record, now time.Time, dryRun bool) int {
	root, err := state.RootDir()
	if err != nil {
		return 0
	}
	result, sweepErr := reaper.SweepParallelDirs(
		filepath.Join(root, "parallel"), live, staleParallelDirMaxAge, now, dryRun)
	if sweepErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not sweep parallel state dirs: %v\n", sweepErr)
	}
	for _, path := range result.Paths {
		diag.Printf("stale state dir: %s\n", path)
	}
	return result.Removed
}
