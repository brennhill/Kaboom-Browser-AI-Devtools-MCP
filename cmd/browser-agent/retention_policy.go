// retention_policy.go — The daemon's disk budgets and the sweep that applies them.
// Why: nothing ever expired a captured artifact. One developer state directory
// reached 1.0GB: 5,975 screenshots, 5,317 recordings, 191,923 project files, and a
// 10.5MB single diagnostics log. Each directory now has a ceiling on count, bytes,
// and age, applied on a timer rather than only when someone notices.
// Docs: docs/core/reliability/zombie-prevention.md

package main

import (
	"context"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/retention"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// retentionSweepInterval is how often budgets are applied. Hourly is frequent
// enough to bound growth and rare enough to be invisible on the hot path.
const retentionSweepInterval = time.Hour

// captureBudget names a directory and its ceiling.
type captureBudget struct {
	Name   string
	Dir    func() (string, error)
	Budget retention.Budget
}

// captureBudgets is the full set of budgeted directories. Every budget bounds all
// three dimensions: a count ceiling alone still lets a few enormous files fill a
// disk, and a byte ceiling alone still lets a million tiny ones exhaust inodes.
func captureBudgets() []captureBudget {
	return []captureBudget{
		{
			Name: "screenshots", Dir: state.ScreenshotsDir,
			Budget: retention.Budget{MaxFiles: 500, MaxBytes: 256 << 20, MaxAge: 7 * 24 * time.Hour},
		},
		{
			Name: "recordings", Dir: state.RecordingsDir,
			Budget: retention.Budget{MaxFiles: 500, MaxBytes: 256 << 20, MaxAge: 30 * 24 * time.Hour},
		},
		{
			Name: "performance-traces", Dir: state.PerformanceTracesDir,
			// Traces are ~21MB each; six of them held 131MB on one machine.
			Budget: retention.Budget{MaxFiles: 20, MaxBytes: 256 << 20, MaxAge: 7 * 24 * time.Hour},
		},
		{
			Name: "evidence", Dir: state.EvidenceDir,
			Budget: retention.Budget{MaxFiles: 500, MaxBytes: 128 << 20, MaxAge: 7 * 24 * time.Hour},
		},
		{
			Name: "logs", Dir: state.LogsDir,
			Budget: retention.Budget{MaxFiles: 50, MaxBytes: 128 << 20, MaxAge: 14 * 24 * time.Hour},
		},
	}
}

// startRetentionSweeper applies every budget on a timer until ctx is cancelled.
// A sweep runs immediately at startup so a daemon that has been down while disk
// accumulated does not wait an hour to reclaim it.
func startRetentionSweeper(ctx context.Context, server *Server, port int) {
	go func() {
		sweepCaptureBudgets(server, port)
		ticker := time.NewTicker(retentionSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweepCaptureBudgets(server, port)
			}
		}
	}()
}

func sweepCaptureBudgets(server *Server, port int) {
	now := time.Now()
	for _, budget := range captureBudgets() {
		dir, err := budget.Dir()
		if err != nil {
			server.logLifecycle("retention_dir_unresolved", port, map[string]any{
				"budget": budget.Name, "error": err.Error(),
			})
			continue
		}
		result, sweepErr := retention.Enforce(dir, budget.Budget, now)
		if sweepErr != nil {
			// A sweep failure is reported, never swallowed: silently failing to
			// reclaim is how the directory reached a gigabyte in the first place.
			server.logLifecycle("retention_sweep_failed", port, map[string]any{
				"budget": budget.Name, "error": sweepErr.Error(),
			})
		}
		if result.RemovedFiles > 0 {
			server.logLifecycle("retention_swept", port, map[string]any{
				"budget": budget.Name, "removed_files": result.RemovedFiles,
				"removed_bytes": result.RemovedBytes, "kept_files": result.KeptFiles,
			})
		}
	}
}
