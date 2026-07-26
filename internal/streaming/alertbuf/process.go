// Purpose: Implements alert post-processing (dedup, correlation, sorting, formatting).
// Why: Keeps alert-shaping logic independent from alert producers (buffer, CI, anomaly).
// Docs: docs/features/feature/push-alerts/index.md

package alertbuf

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Alert Processing: Deduplication
// ============================================

// DeduplicateAlerts merges alerts with the same title+category.
func DeduplicateAlerts(alerts []types.Alert) []types.Alert {
	type dedupKey struct {
		title    string
		category string
	}

	seen := make(map[dedupKey]int)
	var result []types.Alert

	for _, a := range alerts {
		key := dedupKey{title: a.Title, category: a.Category}
		if idx, ok := seen[key]; ok {
			result[idx].Count++
			if a.Timestamp > result[idx].Timestamp {
				result[idx].Timestamp = a.Timestamp
			}
		} else {
			a.Count = 1
			seen[key] = len(result)
			result = append(result, a)
		}
	}

	for i := range result {
		if result[i].Count == 1 {
			result[i].Count = 0
		}
	}

	return result
}

// ============================================
// Alert Processing: Correlation
// ============================================

// CorrelateAlerts groups regression+anomaly pairs within 5s.
func CorrelateAlerts(alerts []types.Alert) []types.Alert {
	if len(alerts) < 2 {
		return alerts
	}

	var result []types.Alert
	used := make([]bool, len(alerts))

	for i := 0; i < len(alerts); i++ {
		if used[i] {
			continue
		}
		if partner := findCorrelationPartner(alerts, used, i); partner >= 0 {
			result = append(result, MergeAlerts(alerts[i], alerts[partner]))
			used[i] = true
			used[partner] = true
		} else {
			result = append(result, alerts[i])
			used[i] = true
		}
	}

	return result
}

func findCorrelationPartner(alerts []types.Alert, used []bool, i int) int {
	if alerts[i].Category != "regression" && alerts[i].Category != "anomaly" {
		return -1
	}
	for j := i + 1; j < len(alerts); j++ {
		if !used[j] && CanCorrelate(alerts[i], alerts[j]) {
			return j
		}
	}
	return -1
}

// CanCorrelate checks if two alerts can be correlated (regression+anomaly within window).
func CanCorrelate(a, b types.Alert) bool {
	if (a.Category != "regression" || b.Category != "anomaly") &&
		(a.Category != "anomaly" || b.Category != "regression") {
		return false
	}

	ta, errA := time.Parse(time.RFC3339, a.Timestamp)
	tb, errB := time.Parse(time.RFC3339, b.Timestamp)
	if errA != nil || errB != nil {
		return false
	}

	diff := ta.Sub(tb)
	if diff < 0 {
		diff = -diff
	}
	return diff <= CorrelationWindow
}

// MergeAlerts combines two correlated alerts into one.
func MergeAlerts(a, b types.Alert) types.Alert {
	severity := a.Severity
	if streaming.SeverityRank(b.Severity) > streaming.SeverityRank(a.Severity) {
		severity = b.Severity
	}

	ts := a.Timestamp
	if b.Timestamp > a.Timestamp {
		ts = b.Timestamp
	}

	return types.Alert{
		Severity:  severity,
		Category:  "regression",
		Title:     "Correlated: " + a.Title + " + " + b.Title,
		Detail:    a.Detail + " | " + b.Detail,
		Timestamp: ts,
		Source:    a.Source,
	}
}

// ============================================
// Alert Processing: Priority Sorting
// ============================================

// SortAlertsByPriority sorts alerts by severity (descending) then timestamp (newest first).
func SortAlertsByPriority(alerts []types.Alert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		ri := streaming.SeverityRank(alerts[i].Severity)
		rj := streaming.SeverityRank(alerts[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return alerts[i].Timestamp > alerts[j].Timestamp
	})
}

// ============================================
// Alert Formatting
// ============================================

// FormatAlertsBlock produces the text content block for alerts.
func FormatAlertsBlock(alerts []types.Alert) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- ALERTS (%d) ---\n", len(alerts)))

	if len(alerts) > 3 {
		summary := BuildAlertSummary(alerts)
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	alertsJSON, _ := json.Marshal(alerts)
	sb.Write(alertsJSON)

	return sb.String()
}

// BuildAlertSummary creates a one-line summary like "4 alerts: 1 regression, 2 anomaly, 1 ci".
func BuildAlertSummary(alerts []types.Alert) string {
	categories := make(map[string]int)
	for _, a := range alerts {
		categories[a.Category]++
	}

	var parts []string
	for _, cat := range []string{"regression", "anomaly", "ci", "noise", "threshold"} {
		if count, ok := categories[cat]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", count, cat))
		}
	}

	return fmt.Sprintf("%d alerts: %s", len(alerts), strings.Join(parts, ", "))
}
