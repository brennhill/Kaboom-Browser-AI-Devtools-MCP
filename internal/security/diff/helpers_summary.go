// helpers_summary.go — Severity-categorized summary construction and age formatting.
// Purpose: Builds severity-categorized summaries from security diff regressions and improvements.
// Why: Separates summary construction from individual diff comparison helpers.
package diff

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

func buildSummary(regressions, improvements []Change) Summary {
	bySeverity := make(map[string]int)
	byCategory := make(map[string]int)

	for _, r := range regressions {
		bySeverity[r.Severity]++
		byCategory[r.Category]++
	}

	return Summary{
		TotalRegressions:  len(regressions),
		TotalImprovements: len(improvements),
		BySeverity:        bySeverity,
		ByCategory:        byCategory,
	}
}

// formatDuration delegates to util.FormatDuration for human-readable duration formatting.
func formatDuration(d time.Duration) string {
	return util.FormatDuration(d)
}
