// Purpose: Converts CI result updates into deterministic alert records in AlertBuffer.
// Why: Separates CI-specific alert semantics from generic buffering and correlation logic.
// Docs: docs/features/feature/push-alerts/index.md

package alertbuf

import (
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ProcessCIResult stores the CI result and generates an alert if new.
// Returns the new alert (for streaming), or nil if this was an idempotent update.
func (ab *AlertBuffer) ProcessCIResult(ciResult types.CIResult) *types.Alert {
	ab.Mu.Lock()
	defer ab.Mu.Unlock()

	if ab.updateExistingCIResult(ciResult) {
		return nil
	}

	if len(ab.CIResults) >= CIResultsCap {
		newResults := make([]types.CIResult, len(ab.CIResults)-1)
		copy(newResults, ab.CIResults[1:])
		ab.CIResults = newResults
	}
	ab.CIResults = append(ab.CIResults, ciResult)

	alert := BuildCIAlert(ciResult)
	ab.appendAlertLocked(alert, time.Now())
	return &alert
}

// updateExistingCIResult checks for and updates an existing CI result.
// Must be called with ab.Mu held. Returns true if an existing entry was updated.
func (ab *AlertBuffer) updateExistingCIResult(ciResult types.CIResult) bool {
	for i := range ab.CIResults {
		if ab.CIResults[i].Commit != ciResult.Commit || ab.CIResults[i].Status != ciResult.Status {
			continue
		}
		ab.CIResults[i] = ciResult
		for j, alert := range ab.Alerts {
			if alert.Category == "ci" && strings.Contains(alert.Detail, ciResult.Commit) {
				ab.Alerts[j] = BuildCIAlert(ciResult)
				break
			}
		}
		return true
	}
	return false
}

// BuildCIAlert creates an alert from a CI result.
func BuildCIAlert(ci types.CIResult) types.Alert {
	severity := "info"
	if ci.Status == "failure" || ci.Status == "error" {
		severity = "error"
	}

	detail := ci.Summary
	if len(ci.Failures) > 0 {
		failNames := make([]string, 0, len(ci.Failures))
		for _, f := range ci.Failures {
			failNames = append(failNames, f.Name)
		}
		detail += " | Failed: " + strings.Join(failNames, ", ")
	}
	if ci.Commit != "" {
		detail += " [" + ci.Commit + "]"
	}

	return types.Alert{
		Severity:  severity,
		Category:  "ci",
		Title:     fmt.Sprintf("CI %s (%s)", ci.Status, ci.Source),
		Detail:    detail,
		Timestamp: ci.ReceivedAt.Format(time.RFC3339),
		Source:    "ci_webhook",
	}
}
