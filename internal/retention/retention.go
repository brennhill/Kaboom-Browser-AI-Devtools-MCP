// retention.go — Bounds how much disk a capture directory may hold.
// Why: nothing ever expired captured artifacts. One developer state directory held
// 5,975 screenshots, 5,317 recordings, and 166,824 project directories totalling
// 1.0GB. Budgets are applied oldest-first in a single pass, per this repository's
// rule against loop-remove-recheck eviction.
// Docs: docs/core/reliability/zombie-prevention.md

package retention

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Budget is the ceiling for one directory. A zero field disables that limit, and
// a wholly zero Budget is a no-op: a missing configuration must never be read as
// permission to delete everything.
type Budget struct {
	MaxFiles int
	MaxBytes int64
	MaxAge   time.Duration
}

// IsZero reports whether this budget constrains anything.
func (b Budget) IsZero() bool {
	return b.MaxFiles <= 0 && b.MaxBytes <= 0 && b.MaxAge <= 0
}

// Result reports what one Enforce call reclaimed.
type Result struct {
	RemovedFiles int
	RemovedBytes int64
	KeptFiles    int
	KeptBytes    int64
}

type entry struct {
	path    string
	size    int64
	modTime time.Time
}

// Enforce applies budget to the files directly inside dir, removing the oldest
// first until every limit is satisfied. Subdirectories are never removed and are
// not descended into: a capture directory's own layout is not this package's to
// reorganize.
//
// A missing directory is not an error — the budget is trivially satisfied.
func Enforce(dir string, budget Budget, now time.Time) (Result, error) {
	if budget.IsZero() {
		return Result{}, nil
	}
	entries, totalBytes, err := scan(dir)
	if err != nil {
		return Result{}, err
	}
	if len(entries) == 0 {
		return Result{}, nil
	}

	// Oldest first: eviction order is decided once, before anything is removed.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].modTime.Equal(entries[j].modTime) {
			return entries[i].path < entries[j].path
		}
		return entries[i].modTime.Before(entries[j].modTime)
	})

	remainingFiles := len(entries)
	remainingBytes := totalBytes
	var result Result
	var firstErr error

	// Single pass: each file is visited once, and the decision to drop it is made
	// from running totals rather than by re-measuring the directory after every
	// removal.
	for _, item := range entries {
		if !overBudget(budget, item, remainingFiles, remainingBytes, now) {
			result.KeptFiles++
			result.KeptBytes += item.size
			continue
		}
		if err := os.Remove(item.path); err != nil {
			if firstErr == nil && !errors.Is(err, os.ErrNotExist) {
				firstErr = fmt.Errorf("retention: remove %s: %w", item.path, err)
			}
			result.KeptFiles++
			result.KeptBytes += item.size
			continue
		}
		remainingFiles--
		remainingBytes -= item.size
		result.RemovedFiles++
		result.RemovedBytes += item.size
	}
	return result, firstErr
}

// overBudget reports whether this file must go, given what is still present.
func overBudget(budget Budget, item entry, remainingFiles int, remainingBytes int64, now time.Time) bool {
	if budget.MaxAge > 0 && now.Sub(item.modTime) > budget.MaxAge {
		return true
	}
	if budget.MaxFiles > 0 && remainingFiles > budget.MaxFiles {
		return true
	}
	if budget.MaxBytes > 0 && remainingBytes > budget.MaxBytes {
		return true
	}
	return false
}

func scan(dir string) ([]entry, int64, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// EXPECTED_ABSENCE: a capture directory that was never written is a
			// budget that is already satisfied, not a fault to report.
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("retention: read %s: %w", dir, err)
	}
	entries := make([]entry, 0, len(dirEntries))
	var total int64
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}
		info, infoErr := dirEntry.Info()
		if infoErr != nil {
			// EXPECTED_ABSENCE: a file removed by a concurrent writer between
			// ReadDir and Info is normal churn in a live capture directory.
			continue
		}
		entries = append(entries, entry{
			path:    filepath.Join(dir, dirEntry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		total += info.Size()
	}
	return entries, total, nil
}
