// Purpose: Formats human-readable summaries of test healing results for MCP responses.
// Why: Separates presentation formatting from healing computation and selector analysis.
package heal

import "fmt"

// FormatHealSummary formats a human-readable summary of healing results.
func FormatHealSummary(params TestHealRequest, result any) string {
	switch params.Action {
	case "analyze":
		var count int
		if m, ok := result.(map[string]any); ok {
			count, _ = m["count"].(int)
		}
		return fmt.Sprintf("Found %d selectors in %s", count, params.TestFile)
	case "repair":
		hr := result.(*HealResult)
		return fmt.Sprintf("Healed %d/%d selectors (%d unhealed, %d auto-applied)",
			len(hr.Healed), hr.Summary.TotalBroken, hr.Summary.Unhealed, hr.Summary.HealedAuto)
	case "batch":
		br := result.(*BatchHealResult)
		return fmt.Sprintf("Healed %d/%d selectors across %d files (%d files skipped, %d selectors unhealed)",
			br.TotalHealed, br.TotalSelectors, br.FilesProcessed, br.FilesSkipped, br.TotalUnhealed)
	}
	return ""
}
