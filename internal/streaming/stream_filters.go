// Purpose: Maps event categories to alert types and checks if alerts pass configured filters.
// Why: Separates filter matching from dedup, emission, and rate limiting.
package streaming

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

// eventCategoryMap maps streaming event types to matching alert categories.
var eventCategoryMap = map[string]map[string]bool{
	"errors":           {"anomaly": true, "threshold": true},
	"network_errors":   {"anomaly": true},
	"performance":      {"regression": true},
	"regression":       {"regression": true},
	"anomaly":          {"anomaly": true},
	"ci":               {"ci": true},
	"security":         {"threshold": true},
	"user_frustration": {"anomaly": true},
}

// ShouldEmit checks if an alert passes all configured filters.
func (s *StreamState) ShouldEmit(alert types.Alert) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !s.Config.Enabled {
		return false
	}
	if SeverityRank(alert.Severity) < SeverityRank(s.Config.SeverityMin) {
		return false
	}
	return s.matchesEventFilter(alert.Category)
}

// matchesEventFilter checks if a category matches the configured event filters.
// Must be called with s.Mu held.
func (s *StreamState) matchesEventFilter(category string) bool {
	for _, evt := range s.Config.Events {
		if evt == "all" {
			return true
		}
		if CategoryMatchesEvent(category, evt) {
			return true
		}
	}
	return false
}

// CategoryMatchesEvent maps alert categories to streaming event types.
func CategoryMatchesEvent(category, event string) bool {
	cats, ok := eventCategoryMap[event]
	return ok && cats[category]
}

// SeverityRank returns the numeric rank of a severity string.
// It defines the ordering used by the severity_min filter below and by
// consumers that sort alerts by priority.
func SeverityRank(s string) int {
	switch s {
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// passesFiltersLocked checks enabled, severity, and event filters.
// Caller must hold s.Mu.
func (s *StreamState) passesFiltersLocked(alert types.Alert) bool {
	if !s.Config.Enabled {
		return false
	}
	if SeverityRank(alert.Severity) < SeverityRank(s.Config.SeverityMin) {
		return false
	}
	return s.matchesEventFilter(alert.Category)
}
