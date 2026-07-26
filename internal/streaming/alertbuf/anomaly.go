// Purpose: Tracks error-frequency windows and generates anomaly alerts when spikes occur.
// Why: Isolates anomaly-detection policy from CI and formatting concerns for easier testing.
// Docs: docs/features/feature/push-alerts/index.md

package alertbuf

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// RecordErrorForAnomaly tracks error timestamps for anomaly detection.
// If error frequency exceeds 3x the rolling average in a 10-second window,
// an anomaly alert is generated.
func (ab *AlertBuffer) RecordErrorForAnomaly(t time.Time) {
	newAlert, stream := func() (*types.Alert, *streaming.StreamState) {
		ab.Mu.Lock()
		defer ab.Mu.Unlock()

		ab.ErrorTimes = append(ab.ErrorTimes, t)
		ab.pruneErrorTimes(t)

		if len(ab.ErrorTimes) < 2 {
			return nil, nil
		}

		recentCount := ab.countRecentErrors(t)
		rollingAvg := float64(len(ab.ErrorTimes)) / (float64(AnomalyWindowSeconds) / float64(AnomalyBucketSeconds))
		return ab.maybeCreateAnomalyAlert(t, recentCount, rollingAvg), ab.Stream
	}()
	if newAlert != nil && stream != nil {
		stream.EmitAlert(*newAlert)
	}
}

// pruneErrorTimes removes timestamps older than the anomaly window.
// Must be called with ab.Mu held.
func (ab *AlertBuffer) pruneErrorTimes(t time.Time) {
	cutoff := t.Add(-time.Duration(AnomalyWindowSeconds) * time.Second)
	pruned := make([]time.Time, 0, len(ab.ErrorTimes))
	for _, et := range ab.ErrorTimes {
		if et.After(cutoff) {
			pruned = append(pruned, et)
		}
	}
	ab.ErrorTimes = pruned
}

// countRecentErrors counts errors within the anomaly bucket window.
// Must be called with ab.Mu held.
func (ab *AlertBuffer) countRecentErrors(t time.Time) int {
	recentCutoff := t.Add(-time.Duration(AnomalyBucketSeconds) * time.Second)
	count := 0
	for _, et := range ab.ErrorTimes {
		if et.After(recentCutoff) {
			count++
		}
	}
	return count
}

// maybeCreateAnomalyAlert checks for a spike and creates an alert if warranted.
// Must be called with ab.Mu held. Returns the new alert or nil.
func (ab *AlertBuffer) maybeCreateAnomalyAlert(t time.Time, recentCount int, rollingAvg float64) *types.Alert {
	if rollingAvg <= 0 || float64(recentCount) <= 3.0*rollingAvg {
		return nil
	}

	if ab.hasRecentAnomalyAlert(t) {
		return nil
	}

	alert := types.Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "Error frequency spike detected",
		Detail:    fmt.Sprintf("%d errors in last %ds vs %.1f rolling average", recentCount, AnomalyBucketSeconds, rollingAvg),
		Timestamp: t.Format(time.RFC3339),
		Source:    "anomaly_detector",
	}
	if len(ab.Alerts) >= AlertBufferCap {
		newAlerts := make([]types.Alert, len(ab.Alerts)-1)
		copy(newAlerts, ab.Alerts[1:])
		ab.Alerts = newAlerts
	}
	ab.Alerts = append(ab.Alerts, alert)
	return &alert
}

// hasRecentAnomalyAlert checks if an anomaly alert was already generated recently.
// Must be called with ab.Mu held.
func (ab *AlertBuffer) hasRecentAnomalyAlert(t time.Time) bool {
	for _, a := range ab.Alerts {
		if a.Category != "anomaly" || a.Source != "anomaly_detector" {
			continue
		}
		at, err := time.Parse(time.RFC3339, a.Timestamp)
		if err != nil {
			continue
		}
		if t.Sub(at) < time.Duration(AnomalyBucketSeconds)*time.Second {
			return true
		}
	}
	return false
}
