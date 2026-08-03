// Purpose: Atomically checks all filters and emits MCP notifications for qualifying alerts.
// Why: Separates the emit path from dedup, filter, and rate-limit logic.
package streaming

import (
	"encoding/json"
	"io"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// EmitAlert atomically checks all filters and emits an MCP notification if appropriate.
func (s *StreamState) EmitAlert(alert types.Alert) {
	type emitPlan struct {
		emit         bool
		writer       io.Writer
		notification MCPNotification
	}
	plan := func() emitPlan {
		s.Mu.Lock()
		defer s.Mu.Unlock()

		if !s.passesFiltersLocked(alert) {
			return emitPlan{}
		}

		now := time.Now()
		if !s.canEmitAtLocked(now) {
			if len(s.PendingBatch) < MaxPendingBatch {
				if len(s.PendingBatch) == 0 {
					s.PendingSince = now
				}
				s.PendingBatch = append(s.PendingBatch, alert)
			} else {
				s.DroppedCount++
			}
			return emitPlan{}
		}

		dedupKey := alert.Category + ":" + alert.Title
		if s.isDuplicateLocked(dedupKey, now) {
			return emitPlan{}
		}

		s.recordEmissionLocked(dedupKey, now)
		return emitPlan{
			emit:         true,
			writer:       s.Writer,
			notification: FormatMCPNotification(alert),
		}
	}()
	if !plan.emit {
		return
	}

	if plan.writer != nil {
		data, err := json.Marshal(plan.notification)
		if err == nil {
			_, _ = plan.writer.Write(data)
			_, _ = plan.writer.Write([]byte{'\n'})
		}
	}
}

// Pressure returns the pending notification queue's bounded resource state.
func (s *StreamState) Pressure() PressureSnapshot {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	pressure := PressureSnapshot{
		Size: len(s.PendingBatch), Capacity: MaxPendingBatch, Dropped: s.DroppedCount,
		Saturated: s.DroppedCount > 0,
	}
	if !s.PendingSince.IsZero() && len(s.PendingBatch) > 0 {
		pressure.OldestAge = time.Since(s.PendingSince)
		if pressure.OldestAge < 0 {
			pressure.OldestAge = 0
		}
	}
	return pressure
}

// FormatMCPNotification creates an MCP notification from an alert.
func FormatMCPNotification(alert types.Alert) MCPNotification {
	return MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/message",
		Params: NotificationParams{
			Level:  alert.Severity,
			Logger: NotificationLoggerName,
			Data: map[string]any{
				"category":  alert.Category,
				"severity":  alert.Severity,
				"title":     alert.Title,
				"detail":    alert.Detail,
				"timestamp": alert.Timestamp,
				"source":    alert.Source,
			},
		},
	}
}
