// Purpose: Owns base AlertBuffer append/drain behavior independent of CI/anomaly generation logic.
// Why: Keeps core buffering semantics small and testable while other alert producers remain modular.
// Docs: docs/features/feature/push-alerts/index.md

package alertbuf

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"
)

// AddAlert appends an alert to the buffer, evicting the oldest if at capacity.
// Also emits the alert as an MCP notification if streaming is enabled.
func (ab *AlertBuffer) AddAlert(a types.Alert) {
	stream := func() *streaming.StreamState {
		ab.Mu.Lock()
		defer ab.Mu.Unlock()
		ab.appendAlertLocked(a, time.Now())
		return ab.Stream
	}()

	if stream != nil {
		stream.EmitAlert(a)
	}
}

func (ab *AlertBuffer) appendAlertLocked(alert types.Alert, now time.Time) {
	for len(ab.AlertTimes) < len(ab.Alerts) {
		ab.AlertTimes = append(ab.AlertTimes, now)
	}
	if len(ab.Alerts) >= AlertBufferCap {
		kept := make([]types.Alert, len(ab.Alerts)-1)
		copy(kept, ab.Alerts[1:])
		ab.Alerts = kept
		ab.AlertTimes = ab.AlertTimes[1:]
		ab.Dropped++
	}
	ab.Alerts = append(ab.Alerts, alert)
	ab.AlertTimes = append(ab.AlertTimes, now)
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
		ab.AlertTimes = nil
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

// Pressure returns the alert buffer's bounded retention metrics.
func (ab *AlertBuffer) Pressure() streaming.PressureSnapshot {
	ab.Mu.Lock()
	defer ab.Mu.Unlock()
	pressure := streaming.PressureSnapshot{
		Size: len(ab.Alerts), Capacity: AlertBufferCap, Dropped: ab.Dropped, Saturated: ab.Dropped > 0,
	}
	if len(ab.AlertTimes) > 0 {
		pressure.OldestAge = time.Since(ab.AlertTimes[0])
	}
	return pressure
}
