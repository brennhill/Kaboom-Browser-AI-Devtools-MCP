// Purpose: Counts action types and builds type-value maps for recording diff analysis.
// Why: Provides reusable helper functions shared by diff comparison and report generation.
package logdiff

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"

// CountActionTypes returns the number of error, click, type, and navigate
// actions in the slice.
func CountActionTypes(actions []recording.RecordingAction) (errors, clicks, types, navigates int) {
	for _, action := range actions {
		switch action.Type {
		case "error":
			errors++
		case "click":
			clicks++
		case "type":
			types++
		case "navigate":
			navigates++
		}
	}
	return
}

// BuildTypeValueMap maps each typed-into selector to the last text typed there.
func BuildTypeValueMap(actions []recording.RecordingAction) map[string]string {
	values := make(map[string]string)
	for _, action := range actions {
		if action.Type == "type" && action.Selector != "" {
			values[action.Selector] = action.Text
		}
	}
	return values
}

// CategorizeActionTypes counts every action type present in a recording.
func CategorizeActionTypes(rec *recording.Recording) map[string]int {
	counts := make(map[string]int)
	for _, action := range rec.Actions {
		counts[action.Type]++
	}
	return counts
}
