// Purpose: Compares original and replay recordings to produce a regression diff report.
// Why: Separates high-level diff orchestration from helper utilities and report formatting.
package logdiff

import (
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

// Compare loads both recordings and diffs the replay against the original.
func Compare(src RecordingSource, originalRecordingID, replayRecordingID string) (*Result, error) {
	original, err := src.GetRecording(originalRecordingID)
	if err != nil {
		return nil, fmt.Errorf("logdiff_load_original_failed: Failed to load original recording: %w", err)
	}

	replay, err := src.GetRecording(replayRecordingID)
	if err != nil {
		return nil, fmt.Errorf("logdiff_load_replay_failed: Failed to load replay recording: %w", err)
	}

	result := &Result{
		OriginalRecording: originalRecordingID,
		ReplayRecording:   replayRecordingID,
		NewErrors:         make([]LogEntry, 0),
		MissingEvents:     make([]LogEntry, 0),
		ChangedValues:     make([]ValueChange, 0),
	}

	result.ActionStats = compareActions(original, replay)
	detectRegressions(original, replay, result)
	detectFixes(original, replay, result)
	detectValueChanges(original, replay, result)
	determineStatus(result)

	return result, nil
}

func compareActions(original, replay *recording.Recording) ActionComparison {
	stats := ActionComparison{
		OriginalCount: original.ActionCount,
		ReplayCount:   replay.ActionCount,
	}
	stats.ErrorsOriginal, stats.ClicksOriginal, stats.TypesOriginal, stats.NavigatesOriginal = CountActionTypes(original.Actions)
	stats.ErrorsReplay, stats.ClicksReplay, stats.TypesReplay, stats.NavigatesReplay = CountActionTypes(replay.Actions)
	return stats
}

// collectUniqueErrors returns high-severity LogEntries for error actions in
// candidate whose text never appears as an error in baseline.
func collectUniqueErrors(baseline, candidate *recording.Recording) []LogEntry {
	known := make(map[string]bool)
	for _, action := range baseline.Actions {
		if action.Type == "error" {
			known[action.Text] = true
		}
	}

	unique := make([]LogEntry, 0)
	for _, action := range candidate.Actions {
		if action.Type == "error" && !known[action.Text] {
			unique = append(unique, LogEntry{
				Type:       "error",
				Severity:   "high",
				Level:      "error",
				Message:    action.Text,
				Timestamp:  action.TimestampMs,
				Selector:   action.Selector,
				ActionType: action.Type,
			})
		}
	}
	return unique
}

func detectRegressions(original, replay *recording.Recording, result *Result) {
	result.NewErrors = append(result.NewErrors, collectUniqueErrors(original, replay)...)
}

func detectFixes(original, replay *recording.Recording, result *Result) {
	result.MissingEvents = append(result.MissingEvents, collectUniqueErrors(replay, original)...)
}

func detectValueChanges(original, replay *recording.Recording, result *Result) {
	originalValues := BuildTypeValueMap(original.Actions)

	for _, action := range replay.Actions {
		if action.Type != "type" || action.Selector == "" {
			continue
		}
		originalValue, exists := originalValues[action.Selector]
		if !exists || originalValue == action.Text {
			continue
		}
		result.ChangedValues = append(result.ChangedValues, ValueChange{
			Field:     action.Selector,
			FromValue: originalValue,
			ToValue:   action.Text,
			Timestamp: action.TimestampMs,
		})
	}
}

func determineStatus(result *Result) {
	if len(result.NewErrors) > 0 {
		result.Status = "regression"
		result.Summary = fmt.Sprintf("⚠️ REGRESSION: %d new errors detected", len(result.NewErrors))
		return
	}

	if len(result.MissingEvents) > 0 && len(result.NewErrors) == 0 {
		result.Status = "fixed"
		result.Summary = fmt.Sprintf("✓ FIXED: %d errors no longer appear", len(result.MissingEvents))
		return
	}

	if len(result.ChangedValues) > 0 {
		result.Status = "changed"
		result.Summary = fmt.Sprintf("⚠️ VALUE CHANGES: %d field(s) changed", len(result.ChangedValues))
		return
	}

	result.Status = "match"
	result.Summary = "All logs match (0 new errors, 0 missing events)"
}
