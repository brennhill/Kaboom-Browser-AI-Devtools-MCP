// Purpose: Formats log diff results into a human-readable regression report string.
// Why: Separates report rendering from diff computation and helper utilities.
package logdiff

import "fmt"

// GetRegressionReport renders the diff as human-readable report text.
func (result *Result) GetRegressionReport() string {
	report := "Log Diff Report\n"
	report += "===============\n"
	report += fmt.Sprintf("Status: %s\n", result.Status)
	report += fmt.Sprintf("Summary: %s\n\n", result.Summary)

	report += "Action Statistics:\n"
	report += fmt.Sprintf("  Original: %d actions\n", result.ActionStats.OriginalCount)
	report += fmt.Sprintf("    - Errors: %d\n", result.ActionStats.ErrorsOriginal)
	report += fmt.Sprintf("    - Clicks: %d\n", result.ActionStats.ClicksOriginal)
	report += fmt.Sprintf("    - Types: %d\n", result.ActionStats.TypesOriginal)
	report += fmt.Sprintf("    - Navigates: %d\n", result.ActionStats.NavigatesOriginal)

	report += fmt.Sprintf("  Replay: %d actions\n", result.ActionStats.ReplayCount)
	report += fmt.Sprintf("    - Errors: %d\n", result.ActionStats.ErrorsReplay)
	report += fmt.Sprintf("    - Clicks: %d\n", result.ActionStats.ClicksReplay)
	report += fmt.Sprintf("    - Types: %d\n", result.ActionStats.TypesReplay)
	report += fmt.Sprintf("    - Navigates: %d\n", result.ActionStats.NavigatesReplay)

	if len(result.NewErrors) > 0 {
		report += fmt.Sprintf("\nNew Errors (%d):\n", len(result.NewErrors))
		for i, err := range result.NewErrors {
			report += fmt.Sprintf("  %d. %s (at %dms)\n", i+1, err.Message, err.Timestamp)
		}
	}

	if len(result.MissingEvents) > 0 {
		report += fmt.Sprintf("\nFixed/Missing Events (%d):\n", len(result.MissingEvents))
		for i, event := range result.MissingEvents {
			report += fmt.Sprintf("  %d. %s (was at %dms)\n", i+1, event.Message, event.Timestamp)
		}
	}

	if len(result.ChangedValues) > 0 {
		report += fmt.Sprintf("\nChanged Values (%d):\n", len(result.ChangedValues))
		for i, change := range result.ChangedValues {
			report += fmt.Sprintf("  %d. %s: '%s' → '%s'\n", i+1, change.Field, change.FromValue, change.ToValue)
		}
	}

	return report
}
