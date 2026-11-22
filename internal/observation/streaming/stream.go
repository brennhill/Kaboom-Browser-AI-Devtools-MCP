// Purpose: Implements streaming configuration, filtering, batching, and alert emission behavior.
// Why: Controls notification throughput and relevance for real-time alert consumers.
// Docs: docs/features/feature/push-alerts/index.md

package streaming

import "time"

// NewStreamState creates a new StreamState with defaults.
func NewStreamState() *StreamState {
	return &StreamState{
		Config: StreamConfig{
			Enabled:         false,
			Events:          []string{"all"},
			ThrottleSeconds: DefaultThrottleSeconds,
			SeverityMin:     DefaultSeverityMin,
		},
		SeenMessages: make(map[string]time.Time),
		MinuteStart:  time.Now(),
		Writer:       nil, // CRITICAL: Never write to stdout in MCP mode (causes parse errors in IDEs)
	}
}

// Configure handles streaming configuration actions (enable/disable/status).
func (s *StreamState) Configure(action string, events []string, throttle int, urlFilter string, severityMin string) map[string]any {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	switch action {
	case "enable":
		s.applyEnableConfig(events, throttle, urlFilter, severityMin)
		return map[string]any{"status": "enabled", "config": s.Config}

	case "disable":
		s.Config.Enabled = false
		pendingCount := len(s.PendingBatch)
		s.PendingBatch = nil
		s.PendingSince = time.Time{}
		s.SeenMessages = make(map[string]time.Time)
		s.NotifyCount = 0
		return map[string]any{"status": "disabled", "pending_cleared": pendingCount}

	case "status":
		return map[string]any{
			"config":       s.Config,
			"notify_count": s.NotifyCount,
			"pending":      len(s.PendingBatch),
		}

	default:
		return map[string]any{
			"error": "unknown action: " + action + ". Use enable, disable, or status.",
		}
	}
}

// applyEnableConfig updates the stream config. Caller must hold s.Mu.
func (s *StreamState) applyEnableConfig(events []string, throttle int, urlFilter string, severityMin string) {
	s.Config.Enabled = true
	s.Config.Events = orDefault(events, []string{"all"})
	s.Config.ThrottleSeconds = orDefaultInt(throttle, DefaultThrottleSeconds)
	if urlFilter != "" {
		s.Config.URLFilter = urlFilter
	}
	s.Config.SeverityMin = orDefaultStr(severityMin, DefaultSeverityMin)
}

func orDefault(val []string, def []string) []string {
	if len(val) > 0 {
		return val
	}
	return def
}

func orDefaultInt(val int, def int) int {
	if val > 0 {
		return val
	}
	return def
}

func orDefaultStr(val string, def string) string {
	if val != "" {
		return val
	}
	return def
}
