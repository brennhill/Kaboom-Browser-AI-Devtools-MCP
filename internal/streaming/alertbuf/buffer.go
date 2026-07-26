// Purpose: Owns base AlertBuffer append/drain behavior independent of CI/anomaly generation logic.
// Why: Keeps core buffering semantics small and testable while other alert producers remain modular.
// Docs: docs/features/feature/push-alerts/index.md

package alertbuf

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// AddAlert appends an alert to the buffer, evicting the oldest if at capacity.
// Also emits the alert as an MCP notification if streaming is enabled.
func (ab *AlertBuffer) AddAlert(a types.Alert) {
	stream := func() *streaming.StreamState {
		ab.Mu.Lock()
		defer ab.Mu.Unlock()
		if len(ab.Alerts) >= AlertBufferCap {
			newAlerts := make([]types.Alert, len(ab.Alerts)-1)
			copy(newAlerts, ab.Alerts[1:])
			ab.Alerts = newAlerts
		}
		ab.Alerts = append(ab.Alerts, a)
		return ab.Stream
	}()

	if stream != nil {
		stream.EmitAlert(a)
	}
}

// DrainAlerts returns all pending alerts (deduplicated, correlated, sorted)
// and clears the buffer. Returns nil if no alerts pending.
func (ab *AlertBuffer) DrainAlerts() []types.Alert {
	raw := func() []types.Alert {
		ab.Mu.Lock()
		defer ab.Mu.Unlock()
		if len(ab.Alerts) == 0 {
			return nil
		}
		out := make([]types.Alert, len(ab.Alerts))
		copy(out, ab.Alerts)
		ab.Alerts = nil
		return out
	}()
	if len(raw) == 0 {
		return nil
	}

	deduped := DeduplicateAlerts(raw)
	correlated := CorrelateAlerts(deduped)
	SortAlertsByPriority(correlated)
	return correlated
}
