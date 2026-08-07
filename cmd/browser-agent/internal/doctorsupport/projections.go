// projections.go — Projects recovery and operational incidents into bounded Doctor checks.
// Docs: docs/features/feature/operational-observability/index.md

package doctorsupport

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

type recoveryDiagnostics interface {
	Snapshot() []statediag.Diagnostic
	Stats() statediag.CollectorStats
}

// Checks projects state recovery and operational incidents into one Doctor surface.
func Checks(recovery recoveryDiagnostics, incidents *incident.Store) []health.DoctorCheck {
	checks := recoveryChecks(recovery)
	return append(checks, incidentChecks(incidents)...)
}

func recoveryChecks(diagnostics recoveryDiagnostics) []health.DoctorCheck {
	if diagnostics == nil {
		return nil
	}
	snapshot := diagnostics.Snapshot()
	checks := make([]health.DoctorCheck, 0, len(snapshot))
	for _, diagnostic := range snapshot {
		status := "warn"
		if diagnostic.Lifecycle == statediag.LifecycleRecovered {
			status = "pass"
		}
		history := make([]health.DoctorTransition, 0, len(diagnostic.History))
		for _, transition := range diagnostic.History {
			history = append(history, health.DoctorTransition{
				Lifecycle: string(transition.Lifecycle), At: transition.At.Format(time.RFC3339Nano),
				Event: transition.Event, CorrelationID: transition.CorrelationID, Outcome: transition.Outcome,
			})
		}
		checks = append(checks, health.DoctorCheck{
			Name: diagnostic.Name, CorrelationID: diagnostic.CorrelationID, Status: status, Detail: diagnostic.Detail, Fix: diagnostic.Fix,
			Lifecycle: string(diagnostic.Lifecycle), FirstSeenAt: formatTime(diagnostic.FirstSeenAt), LastSeenAt: formatTime(diagnostic.LastSeenAt),
			RecoveredAt: formatTime(diagnostic.RecoveredAt), Occurrences: diagnostic.Occurrences,
			LastSuccessfulTransition: diagnostic.LastSuccessfulTransition, ExpectedNextTransition: diagnostic.ExpectedNextTransition,
			Deadline: formatTime(diagnostic.Deadline), RecoveryAttempt: diagnostic.RecoveryAttempt,
			RecoveryOutcome: diagnostic.RecoveryOutcome, History: history,
		})
	}
	stats := diagnostics.Stats()
	if stats.DroppedRecovered > 0 {
		checks = append(checks, health.DoctorCheck{
			Name: "state_recovery_retention", Status: "pass", Lifecycle: string(statediag.LifecycleRecovered),
			Detail: fmt.Sprintf("Doctor retained %d recovered incidents and dropped %d oldest recovered incidents at its %d-entry bound.", stats.Recovered, stats.DroppedRecovered, stats.RecoveredLimit),
			Fix:    "No action required; active incidents remain retained.", Occurrences: stats.DroppedRecovered,
		})
	}
	return checks
}

func incidentChecks(diagnostics *incident.Store) []health.DoctorCheck {
	if diagnostics == nil {
		return nil
	}
	views := diagnostics.DoctorSnapshot()
	checks := make([]health.DoctorCheck, 0, len(views)+1)
	for _, view := range views {
		status := "warn"
		if view.State == incident.StateRecovered {
			status = "pass"
		} else if view.State == incident.StateExhausted || view.Severity == incident.SeverityFatal {
			status = "fail"
		}
		detail := view.Detail
		if view.LocalDetail != "" {
			detail += " Local context: " + view.LocalDetail
		}
		history := make([]health.DoctorTransition, 0, len(view.History))
		for _, transition := range view.History {
			history = append(history, health.DoctorTransition{
				Lifecycle: string(transition.State), At: transition.At.Format(time.RFC3339Nano), Event: string(transition.State),
				CorrelationID: view.CorrelationID, Outcome: string(incidentOutcome(transition.State)),
			})
		}
		recoveredAt := ""
		if view.State == incident.StateRecovered {
			recoveredAt = formatTime(view.ResolvedAt)
		}
		checks = append(checks, health.DoctorCheck{
			Name: string(view.Code), CorrelationID: view.CorrelationID, Fingerprint: view.Fingerprint, Status: status,
			Detail: detail, Fix: view.Fix, Lifecycle: string(view.State), FirstSeenAt: formatTime(view.DetectedAt),
			LastSeenAt: formatTime(view.UpdatedAt), RecoveredAt: recoveredAt, RecoveryAttempt: boundedInt(uint64(view.Attempts)),
			RecoveryOutcome: string(incidentOutcome(view.State)), History: history,
		})
	}
	stats := diagnostics.Stats()
	if stats.Dropped > 0 {
		checks = append(checks, health.DoctorCheck{
			Name: "operational_incident_retention", Status: "warn", Lifecycle: "capacity",
			Detail: fmt.Sprintf("Doctor retained %d operational incidents and dropped %d entries at its %d-entry bound.", stats.Active+stats.Terminal, stats.Dropped, stats.Capacity),
			Fix:    "Inspect recurring incidents and resource pressure before increasing retention.", Occurrences: boundedInt(stats.Dropped),
		})
	}
	return checks
}

func boundedInt(value uint64) int {
	const maxInt = int(^uint(0) >> 1)
	if value > uint64(maxInt) {
		return maxInt
	}
	return int(value)
}

func incidentOutcome(state incident.State) incident.Outcome {
	if state == incident.StateRecovered {
		return incident.OutcomeRecovered
	}
	if state == incident.StateExhausted {
		return incident.OutcomeExhausted
	}
	return incident.OutcomePending
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
