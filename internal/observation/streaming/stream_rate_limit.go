// Purpose: Enforces per-second throttle limits on streaming notifications with burst tracking.
// Why: Separates rate-limit state from dedup, filter matching, and emission logic.
package streaming

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
)

// CanEmitAt checks if enough time has passed since the last notification.
func (s *StreamState) CanEmitAt(now time.Time) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !s.LastNotified.IsZero() {
		elapsed := now.Sub(s.LastNotified)
		if elapsed < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			return false
		}
	}
	s.checkRateResetLocked(now)
	return s.NotifyCount < MaxNotificationsPerMinute
}

// RecordEmission records that a notification was sent.
func (s *StreamState) RecordEmission(now time.Time, _ types.Alert) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
}

// CheckRateReset resets the per-minute counter if a new minute has started.
func (s *StreamState) CheckRateReset(now time.Time) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.checkRateResetLocked(now)
}

// checkRateResetLocked resets counter (caller must hold Mu).
func (s *StreamState) checkRateResetLocked(now time.Time) {
	if now.Sub(s.MinuteStart) >= time.Minute {
		s.NotifyCount = 0
		s.MinuteStart = now
	}
}

// canEmitAtLocked checks throttle and rate limit constraints.
// Caller must hold s.Mu.
func (s *StreamState) canEmitAtLocked(now time.Time) bool {
	if !s.LastNotified.IsZero() {
		if now.Sub(s.LastNotified) < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			return false
		}
	}
	s.checkRateResetLocked(now)
	return s.NotifyCount < MaxNotificationsPerMinute
}
