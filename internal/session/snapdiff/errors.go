// Purpose: Computes error diffs, performance regression counts, and summary verdicts between snapshots.
// Docs: docs/features/feature/request-session-correlation/index.md

// errors.go — Console error diff computation plus the aggregate verdict.
package snapdiff

// Errors computes the set difference of console errors between two snapshots.
func Errors(a, b *NamedSnapshot) ErrorDiff {
	diff := ErrorDiff{
		New:       make([]SnapshotError, 0),
		Resolved:  make([]SnapshotError, 0),
		Unchanged: make([]SnapshotError, 0),
	}

	aMessages := make(map[string]SnapshotError)
	for _, e := range a.ConsoleErrors {
		aMessages[e.Message] = e
	}

	bMessages := make(map[string]SnapshotError)
	for _, e := range b.ConsoleErrors {
		bMessages[e.Message] = e
	}

	// New = in B but not in A
	for msg, e := range bMessages {
		if _, found := aMessages[msg]; !found {
			diff.New = append(diff.New, e)
		}
	}

	// Resolved = in A but not in B
	for msg, e := range aMessages {
		if _, found := bMessages[msg]; !found {
			diff.Resolved = append(diff.Resolved, e)
		} else {
			diff.Unchanged = append(diff.Unchanged, e)
		}
	}

	return diff
}

// countPerfRegressions counts how many performance metrics regressed.
func countPerfRegressions(perf PerformanceDiff) int {
	count := 0
	for _, mc := range []*MetricChange{perf.LoadTime, perf.RequestCount, perf.TransferSize} {
		if mc != nil && mc.Regression {
			count++
		}
	}
	return count
}

// hasStatusRegression returns true if any status change went from OK to error.
func hasStatusRegression(changes []NetworkChange) bool {
	for _, sc := range changes {
		if sc.AfterStatus >= 400 && sc.BeforeStatus < 400 {
			return true
		}
	}
	return false
}

// Summarize derives the verdict and aggregate counts from a computed diff.
func Summarize(result *Result) Summary {
	summary := Summary{
		NewErrors:              len(result.Errors.New),
		ResolvedErrors:         len(result.Errors.Resolved),
		NewNetworkErrors:       len(result.Network.NewErrors),
		PerformanceRegressions: countPerfRegressions(result.Performance),
	}

	hasRegressions := summary.NewErrors > 0 || summary.PerformanceRegressions > 0 ||
		summary.NewNetworkErrors > 0 || hasStatusRegression(result.Network.StatusChanges)
	hasImprovements := summary.ResolvedErrors > 0

	switch {
	case hasRegressions && hasImprovements:
		summary.Verdict = "mixed"
	case hasRegressions:
		summary.Verdict = "regressed"
	case hasImprovements:
		summary.Verdict = "improved"
	default:
		summary.Verdict = "unchanged"
	}

	return summary
}
