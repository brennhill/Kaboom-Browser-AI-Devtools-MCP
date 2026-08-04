// Purpose: Orchestrates full diff between two named snapshots (errors, network, performance, summary).
// Docs: docs/features/feature/request-session-correlation/index.md

// comparison.go — Main comparison logic.
// Compare resolves the two snapshot operands and delegates the arithmetic to snapdiff.
package session

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/snapdiff"
)

// Compare diffs two snapshots. Use "current" as b to compare against live state.
func (sm *SessionManager) Compare(a, b string) (*snapdiff.Result, error) {
	return sm.CompareWithBudgets(a, b, nil)
}

func (sm *SessionManager) CompareWithBudgets(a, b string, budgets map[string]float64) (*snapdiff.Result, error) {
	sm.mu.RLock()
	snapA, existsA := sm.snaps[a]
	sm.mu.RUnlock()

	if !existsA {
		return nil, fmt.Errorf("snapshot %q not found", a)
	}

	var snapB *types.NamedSnapshot
	if b == reservedSnapshotName {
		// Compare against current live state
		snapB = sm.captureCurrentState("current", snapA.URLFilter)
	} else {
		sm.mu.RLock()
		found, exists := sm.snaps[b]
		sm.mu.RUnlock()
		if !exists {
			return nil, fmt.Errorf("snapshot %q not found", b)
		}
		snapB = found
	}

	result := &snapdiff.Result{
		A: a,
		B: b,
	}

	result.Errors = snapdiff.Errors(snapA, snapB)
	result.Network = snapdiff.Network(snapA, snapB)
	result.Performance = snapdiff.PerformanceWithBudgets(snapA, snapB, budgets)
	result.Summary = snapdiff.Summarize(result)

	return result, nil
}
