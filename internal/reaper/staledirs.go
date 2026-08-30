// staledirs.go — Reclaims isolated state directories left behind by finished runs.
// Why: --parallel generates a fresh state directory per launch and nothing ever
// removed one; 880 accumulated on a single developer machine over a month. A
// directory is only eligible when no registered instance still owns it, so a long
// test run is never mistaken for an abandoned one.

package reaper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

// generatedRunDir matches the name shape ApplyParallelStateDir creates:
// run-<unixnano>-<pid>. Matching the SHAPE rather than sweeping every child is
// what keeps a user's own --state-dir from ever being eligible.
var generatedRunDir = regexp.MustCompile(`^run-\d+-\d+$`)

// SweepResult reports what a sweep reclaimed (or, in a dry run, would reclaim).
type SweepResult struct {
	Removed int
	Paths   []string
}

// SweepParallelDirs removes generated run directories under root that are older
// than maxAge and are not claimed by any live instance.
func SweepParallelDirs(root string, live []instancereg.Record, maxAge time.Duration, now time.Time, dryRun bool) (SweepResult, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// EXPECTED_ABSENCE: no parallel root means no parallel run has ever
			// executed here, which is a swept state, not a failure.
			return SweepResult{}, nil
		}
		return SweepResult{}, fmt.Errorf("reaper: read parallel root %s: %w", root, err)
	}

	claimed := make(map[string]bool, len(live))
	for _, record := range live {
		if record.StateDir != "" {
			claimed[filepath.Clean(record.StateDir)] = true
		}
	}

	var result SweepResult
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() || !generatedRunDir.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if claimed[filepath.Clean(path)] {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			// EXPECTED_ABSENCE: a directory removed by a concurrent sweep between
			// ReadDir and Info needs no action and no report.
			continue
		}
		if now.Sub(info.ModTime()) <= maxAge {
			continue
		}
		result.Removed++
		result.Paths = append(result.Paths, path)
		if dryRun {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			failures = append(failures, fmt.Errorf("remove %s: %w", path, err))
		}
	}
	return result, errors.Join(failures...)
}
