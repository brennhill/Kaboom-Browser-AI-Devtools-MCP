// Purpose: Generates concise human-readable performance diff summaries.
// Why: Keeps summary/ranking/string formatting concerns separate from diff computation.
// Docs: docs/features/feature/performance-audit/index.md

package performance

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// metricEntry pairs a metric name with its diff for sorting.
type metricEntry struct {
	name string
	md   MetricDiff
}

// sortedMetrics returns metrics sorted by absolute percentage change (biggest first).
func sortedMetrics(metrics map[string]MetricDiff) []metricEntry {
	sorted := make([]metricEntry, 0, len(metrics))
	for name, md := range metrics {
		sorted = append(sorted, metricEntry{name, md})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return parsePctAbs(sorted[i].md) > parsePctAbs(sorted[j].md)
	})
	return sorted
}

// metricDirection returns "improved", "regressed", or "unchanged" for a metric diff.
func metricDirection(md MetricDiff) string {
	if md.Delta == 0 {
		return "unchanged"
	}
	if md.Improved {
		return "improved"
	}
	return "regressed"
}

// formatTopMetric formats the biggest-change metric for the summary lead.
func formatTopMetric(top metricEntry) string {
	entry := fmt.Sprintf("%s %s %.0f%%", strings.ToUpper(top.name), metricDirection(top.md), parsePctAbs(top.md))
	if top.md.Rating != "" {
		entry += fmt.Sprintf(" (%s)", top.md.Rating)
	}
	return entry
}

// firstRegressionWarning returns a warning string for the first regressed metric, or "".
func firstRegressionWarning(sorted []metricEntry) string {
	for _, entry := range sorted {
		if !entry.md.Improved && entry.md.Delta != 0 {
			return fmt.Sprintf("warning: %s regressed %.0f%%", strings.ToUpper(entry.name), parsePctAbs(entry.md))
		}
	}
	return ""
}

// truncate trims a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// GeneratePerfSummary generates a concise (<200 char) summary of the diff.
// Leads with the biggest improvement, mentions resource changes, flags regressions.
func GeneratePerfSummary(diff PerfDiff) string {
	if len(diff.Metrics) == 0 && len(diff.Resources.Removed) == 0 &&
		len(diff.Resources.Added) == 0 && len(diff.Resources.Resized) == 0 {
		return "No performance changes detected."
	}

	sorted := sortedMetrics(diff.Metrics)
	var parts []string

	if len(sorted) > 0 {
		parts = append(parts, formatTopMetric(sorted[0]))
	}

	if len(diff.Resources.Removed) > 0 {
		parts = append(parts, fmt.Sprintf("removed %s", lastPathSegment(diff.Resources.Removed[0].URL)))
	}

	if warn := firstRegressionWarning(sorted); warn != "" {
		if len(sorted) == 0 || sorted[0].md.Improved {
			parts = append(parts, warn)
		}
	}

	return truncate(strings.Join(parts, "; "), 200)
}

func lastPathSegment(url string) string {
	i := strings.LastIndex(url, "/")
	if i >= 0 && i < len(url)-1 {
		return url[i+1:]
	}
	return url
}

// parsePctAbs extracts the absolute percentage value from metric delta vs baseline.
func parsePctAbs(md MetricDiff) float64 {
	if md.Before == 0 {
		return 0
	}
	return math.Abs(md.Delta / md.Before * 100)
}
