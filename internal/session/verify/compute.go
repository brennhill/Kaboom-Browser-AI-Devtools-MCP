// Purpose: Computes verification diffs (console/network/perf), determines verdict, and dispatches verify_fix MCP actions.
// Docs: docs/features/feature/request-session-correlation/index.md

// verify_compute.go — Verification computation and MCP tool dispatch.
// Contains: computeVerification, determineVerdict, HandleTool.
package verify

import (
	"fmt"
)

// ============================================
// Verification Computation
// ============================================

// sumErrorCounts returns the total error count across all verify errors.
func sumErrorCounts(errors []VerifyError) int {
	total := 0
	for _, e := range errors {
		total += e.Count
	}
	return total
}

// buildIssueSummary creates an IssueSummary from console and network error counts.
func buildIssueSummary(consoleErrors []VerifyError, networkErrors []VerifyNetworkEntry) IssueSummary {
	consoleCount := sumErrorCounts(consoleErrors)
	return IssueSummary{
		ConsoleErrors: consoleCount,
		NetworkErrors: len(networkErrors),
		TotalIssues:   consoleCount + len(networkErrors),
	}
}

// buildVerifyErrorMap indexes verify errors by their normalized message.
func buildVerifyErrorMap(errors []VerifyError) map[string]VerifyError {
	m := make(map[string]VerifyError, len(errors))
	for _, e := range errors {
		m[e.Normalized] = e
	}
	return m
}

// diffConsoleErrors finds resolved and new console errors between before and after snapshots.
func diffConsoleErrors(before, after []VerifyError) (changes, newIssues []VerifyChange) {
	beforeMsgs := buildVerifyErrorMap(before)
	afterMsgs := buildVerifyErrorMap(after)

	for norm, e := range beforeMsgs {
		if _, found := afterMsgs[norm]; found {
			continue
		}
		suffix := ""
		if e.Count > 1 {
			suffix = fmt.Sprintf(" (x%d)", e.Count)
		}
		changes = append(changes, VerifyChange{
			Type: "resolved", Category: "console",
			Before: e.Message + suffix, After: "(not seen)",
		})
	}

	for norm, e := range afterMsgs {
		if _, found := beforeMsgs[norm]; found {
			continue
		}
		newIssues = append(newIssues, VerifyChange{
			Type: "new", Category: "console",
			Before: "(not seen)", After: e.Message,
		})
	}
	return changes, newIssues
}

// formatNetworkEntry formats a network entry for display as "METHOD URL -> STATUS".
func formatNetworkEntry(n VerifyNetworkEntry) string {
	return fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status)
}

// buildNetworkKeyMap indexes network entries by "method path" key.
func buildNetworkKeyMap(entries []VerifyNetworkEntry) map[string]VerifyNetworkEntry {
	m := make(map[string]VerifyNetworkEntry, len(entries))
	for _, n := range entries {
		m[n.Method+" "+n.Path] = n
	}
	return m
}

// classifyNetworkErrorResolution determines the change type for a previously-errored endpoint.
func classifyNetworkErrorResolution(afterEntry VerifyNetworkEntry) string {
	if afterEntry.Status >= 200 && afterEntry.Status < 400 {
		return "resolved"
	}
	return "changed"
}

// diffNetworkErrors finds resolved, changed, and new network errors.
func diffNetworkErrors(before, after *VerifSnapshot) (changes, newIssues []VerifyChange) {
	afterAllNetwork := buildNetworkKeyMap(after.AllNetworkRequests)
	beforeNetwork := buildNetworkKeyMap(before.NetworkErrors)
	afterNetwork := buildNetworkKeyMap(after.NetworkErrors)

	for key, n := range beforeNetwork {
		if afterN, found := afterNetwork[key]; found {
			if afterN.Status != n.Status {
				changes = append(changes, VerifyChange{
					Type: "changed", Category: "network",
					Before: formatNetworkEntry(n), After: formatNetworkEntry(afterN),
				})
			}
			continue
		}
		if allN, found := afterAllNetwork[key]; found {
			changes = append(changes, VerifyChange{
				Type: classifyNetworkErrorResolution(allN), Category: "network",
				Before: formatNetworkEntry(n), After: formatNetworkEntry(allN),
			})
		} else {
			changes = append(changes, VerifyChange{
				Type: "resolved", Category: "network",
				Before: formatNetworkEntry(n), After: "(not seen)",
			})
		}
	}

	for key, n := range afterNetwork {
		if _, found := beforeNetwork[key]; !found {
			newIssues = append(newIssues, VerifyChange{
				Type: "new", Category: "network",
				Before: "(not seen)", After: formatNetworkEntry(n),
			})
		}
	}
	return changes, newIssues
}

// computeLoadTimeDiff computes performance diff between snapshots, or nil if either is missing.
func computeLoadTimeDiff(before, after *VerifSnapshot) *VerifyPerfDiff {
	if before.Performance == nil || after.Performance == nil {
		return nil
	}
	pd := &VerifyPerfDiff{
		LoadTimeBefore: fmt.Sprintf("%.0fms", before.Performance.Timing.Load),
		LoadTimeAfter:  fmt.Sprintf("%.0fms", after.Performance.Timing.Load),
	}
	if before.Performance.Timing.Load > 0 {
		pctChange := ((after.Performance.Timing.Load - before.Performance.Timing.Load) / before.Performance.Timing.Load) * 100
		if pctChange >= 0 {
			pd.Change = fmt.Sprintf("+%.0f%%", pctChange)
		} else {
			pd.Change = fmt.Sprintf("%.0f%%", pctChange)
		}
	}
	return pd
}

// computeVerification compares baseline and after snapshots
func (vm *VerificationManager) computeVerification(before, after *VerifSnapshot) VerificationResult {
	result := VerificationResult{
		Before:    buildIssueSummary(before.ConsoleErrors, before.NetworkErrors),
		After:     buildIssueSummary(after.ConsoleErrors, after.NetworkErrors),
		Changes:   make([]VerifyChange, 0),
		NewIssues: make([]VerifyChange, 0),
	}

	consoleChanges, consoleNew := diffConsoleErrors(before.ConsoleErrors, after.ConsoleErrors)
	result.Changes = append(result.Changes, consoleChanges...)
	result.NewIssues = append(result.NewIssues, consoleNew...)

	networkChanges, networkNew := diffNetworkErrors(before, after)
	result.Changes = append(result.Changes, networkChanges...)
	result.NewIssues = append(result.NewIssues, networkNew...)

	result.PerformanceDiff = computeLoadTimeDiff(before, after)
	result.Verdict = vm.determineVerdict(result)
	return result
}

// countResolvedChanges returns how many changes have type "resolved".
func countResolvedChanges(changes []VerifyChange) int {
	n := 0
	for _, c := range changes {
		if c.Type == "resolved" {
			n++
		}
	}
	return n
}

// determineVerdict determines the overall verdict based on changes
func (vm *VerificationManager) determineVerdict(result VerificationResult) string {
	beforeTotal := result.Before.TotalIssues
	afterTotal := result.After.TotalIssues
	hasChanges := len(result.Changes) > 0
	hasNew := len(result.NewIssues) > 0
	resolvedCount := countResolvedChanges(result.Changes)

	switch {
	case beforeTotal == 0 && afterTotal == 0:
		return "no_issues_detected"
	case resolvedCount > 0 && !hasNew && afterTotal == 0:
		return "fixed"
	case resolvedCount > 0 && !hasNew:
		return "improved"
	case hasChanges && hasNew:
		return "different_issue"
	case hasNew:
		return "regressed"
	default:
		return "unchanged"
	}
}
