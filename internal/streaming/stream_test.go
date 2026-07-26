// Purpose: Tests for SSE streaming connection and event delivery.
// Docs: docs/features/feature/context-streaming/index.md

// stream_test.go — Unit tests for StreamState: config, filters, throttle, dedup, emission.
package streaming

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestSeverityRank(t *testing.T) {
	t.Parallel()

	if got := SeverityRank("error"); got != 3 {
		t.Fatalf("SeverityRank(error) = %d, want 3", got)
	}
	if got := SeverityRank("warning"); got != 2 {
		t.Fatalf("SeverityRank(warning) = %d, want 2", got)
	}
	if got := SeverityRank("info"); got != 1 {
		t.Fatalf("SeverityRank(info) = %d, want 1", got)
	}
	if got := SeverityRank("unknown"); got != 0 {
		t.Fatalf("SeverityRank(unknown) = %d, want 0", got)
	}
}

func TestCategoryMatchesEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		category string
		event    string
		want     bool
	}{
		// "errors" event matches anomaly and threshold
		{"anomaly", "errors", true},
		{"threshold", "errors", true},
		{"regression", "errors", false},
		{"ci", "errors", false},

		// "network_errors" matches anomaly only
		{"anomaly", "network_errors", true},
		{"threshold", "network_errors", false},

		// "performance" and "regression" both match regression
		{"regression", "performance", true},
		{"regression", "regression", true},
		{"anomaly", "performance", false},

		// "anomaly" matches anomaly
		{"anomaly", "anomaly", true},
		{"regression", "anomaly", false},

		// "ci" matches ci
		{"ci", "ci", true},
		{"anomaly", "ci", false},

		// "security" matches threshold
		{"threshold", "security", true},
		{"anomaly", "security", false},

		// "user_frustration" matches anomaly
		{"anomaly", "user_frustration", true},
		{"ci", "user_frustration", false},

		// Unknown event returns false
		{"anomaly", "unknown_event", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got := CategoryMatchesEvent(tt.category, tt.event)
		if got != tt.want {
			t.Errorf("CategoryMatchesEvent(%q, %q) = %v, want %v", tt.category, tt.event, got, tt.want)
		}
	}
}

func TestStreamState_Configure(t *testing.T) {
	t.Parallel()

	t.Run("enable with defaults", func(t *testing.T) {
		s := NewStreamState()
		result := s.Configure("enable", nil, 0, "", "")

		if result["status"] != "enabled" {
			t.Fatalf("expected status=enabled, got %v", result["status"])
		}
		cfg := result["config"].(StreamConfig)
		if !cfg.Enabled {
			t.Fatal("expected Enabled=true")
		}
		if len(cfg.Events) != 1 || cfg.Events[0] != "all" {
			t.Fatalf("expected default events=[all], got %v", cfg.Events)
		}
		if cfg.ThrottleSeconds != DefaultThrottleSeconds {
			t.Fatalf("expected default throttle=%d, got %d", DefaultThrottleSeconds, cfg.ThrottleSeconds)
		}
		if cfg.SeverityMin != DefaultSeverityMin {
			t.Fatalf("expected default severity=%q, got %q", DefaultSeverityMin, cfg.SeverityMin)
		}
	})

	t.Run("enable with custom params", func(t *testing.T) {
		s := NewStreamState()
		result := s.Configure("enable", []string{"errors", "ci"}, 10, "/api", "error")

		cfg := result["config"].(StreamConfig)
		if len(cfg.Events) != 2 || cfg.Events[0] != "errors" || cfg.Events[1] != "ci" {
			t.Fatalf("expected custom events, got %v", cfg.Events)
		}
		if cfg.ThrottleSeconds != 10 {
			t.Fatalf("expected throttle=10, got %d", cfg.ThrottleSeconds)
		}
		if cfg.URLFilter != "/api" {
			t.Fatalf("expected url=/api, got %q", cfg.URLFilter)
		}
		if cfg.SeverityMin != "error" {
			t.Fatalf("expected severity=error, got %q", cfg.SeverityMin)
		}
	})

	t.Run("disable clears state", func(t *testing.T) {
		s := NewStreamState()
		s.Configure("enable", nil, 0, "", "")
		s.PendingBatch = append(s.PendingBatch, types.Alert{Title: "pending"})
		s.SeenMessages["key"] = time.Now()

		result := s.Configure("disable", nil, 0, "", "")

		if result["status"] != "disabled" {
			t.Fatalf("expected status=disabled, got %v", result["status"])
		}
		if result["pending_cleared"] != 1 {
			t.Fatalf("expected pending_cleared=1, got %v", result["pending_cleared"])
		}
		if s.Config.Enabled {
			t.Fatal("expected Enabled=false after disable")
		}
		if len(s.PendingBatch) != 0 {
			t.Fatal("expected PendingBatch cleared")
		}
		if len(s.SeenMessages) != 0 {
			t.Fatal("expected SeenMessages cleared")
		}
	})

	t.Run("status returns current state", func(t *testing.T) {
		s := NewStreamState()
		s.Configure("enable", []string{"ci"}, 0, "", "")
		s.NotifyCount = 5
		s.PendingBatch = make([]types.Alert, 3)

		result := s.Configure("status", nil, 0, "", "")

		cfg := result["config"].(StreamConfig)
		if !cfg.Enabled {
			t.Fatal("status should reflect enabled=true")
		}
		if result["notify_count"] != 5 {
			t.Fatalf("expected notify_count=5, got %v", result["notify_count"])
		}
		if result["pending"] != 3 {
			t.Fatalf("expected pending=3, got %v", result["pending"])
		}
	})

	t.Run("unknown action returns error", func(t *testing.T) {
		s := NewStreamState()
		result := s.Configure("bogus", nil, 0, "", "")

		errMsg, ok := result["error"].(string)
		if !ok || errMsg == "" {
			t.Fatalf("expected error message for unknown action, got %v", result)
		}
	})
}

func TestStreamState_IsDuplicate(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	now := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)

	// First time: not a duplicate
	if s.IsDuplicate("key1", now) {
		t.Fatal("first occurrence should not be duplicate")
	}

	// Record it
	s.RecordDedupKey("key1", now)

	// Within window: duplicate
	if !s.IsDuplicate("key1", now.Add(10*time.Second)) {
		t.Fatal("same key within 30s window should be duplicate")
	}

	// After window: not duplicate
	if s.IsDuplicate("key1", now.Add(31*time.Second)) {
		t.Fatal("same key after 30s window should not be duplicate")
	}

	// Different key: not duplicate
	if s.IsDuplicate("key2", now) {
		t.Fatal("different key should not be duplicate")
	}
}

func TestStreamState_ShouldEmit(t *testing.T) {
	t.Parallel()

	t.Run("disabled returns false", func(t *testing.T) {
		s := NewStreamState()
		alert := types.Alert{Severity: "error", Category: "anomaly"}
		if s.ShouldEmit(alert) {
			t.Fatal("disabled stream should not emit")
		}
	})

	t.Run("severity below minimum filtered out", func(t *testing.T) {
		s := NewStreamState()
		s.Configure("enable", nil, 0, "", "error")

		if s.ShouldEmit(types.Alert{Severity: "warning", Category: "anomaly"}) {
			t.Fatal("warning should be filtered when min=error")
		}
		if !s.ShouldEmit(types.Alert{Severity: "error", Category: "anomaly"}) {
			t.Fatal("error should pass when min=error")
		}
	})

	t.Run("category filter", func(t *testing.T) {
		s := NewStreamState()
		s.Configure("enable", []string{"ci"}, 0, "", "info")

		if s.ShouldEmit(types.Alert{Severity: "error", Category: "anomaly"}) {
			t.Fatal("anomaly should be filtered when events=[ci]")
		}
		if !s.ShouldEmit(types.Alert{Severity: "error", Category: "ci"}) {
			t.Fatal("ci should pass when events=[ci]")
		}
	})

	t.Run("all wildcard passes everything", func(t *testing.T) {
		s := NewStreamState()
		s.Configure("enable", []string{"all"}, 0, "", "info")

		if !s.ShouldEmit(types.Alert{Severity: "info", Category: "regression"}) {
			t.Fatal("all wildcard should pass any category")
		}
	})
}

func TestStreamState_CanEmitAt(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	s.Configure("enable", nil, 5, "", "")

	now := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)

	// First emission: always allowed
	if !s.CanEmitAt(now) {
		t.Fatal("first emission should be allowed")
	}

	// Record emission
	s.RecordEmission(now, types.Alert{})

	// Within throttle window: blocked
	if s.CanEmitAt(now.Add(3 * time.Second)) {
		t.Fatal("should be throttled within 5s window")
	}

	// After throttle window: allowed
	if !s.CanEmitAt(now.Add(6 * time.Second)) {
		t.Fatal("should be allowed after throttle window")
	}
}

func TestStreamState_RateLimit(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	s.Configure("enable", nil, 0, "", "") // zero throttle so only rate limit matters
	s.Config.ThrottleSeconds = 0

	now := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	s.MinuteStart = now

	// Fill up to rate limit
	for i := 0; i < MaxNotificationsPerMinute; i++ {
		s.RecordEmission(now.Add(time.Duration(i)*time.Millisecond), types.Alert{})
	}

	// Should be blocked by rate limit
	if s.CanEmitAt(now.Add(time.Duration(MaxNotificationsPerMinute) * time.Millisecond)) {
		t.Fatal("should be rate limited after max notifications per minute")
	}

	// After minute reset: allowed again
	if !s.CanEmitAt(now.Add(61 * time.Second)) {
		t.Fatal("should be allowed after minute reset")
	}
}

func TestFormatMCPNotification(t *testing.T) {
	t.Parallel()

	alert := types.Alert{
		Severity:  "error",
		Category:  "regression",
		Title:     "test failure",
		Detail:    "details here",
		Timestamp: "2026-02-11T10:00:00Z",
		Source:    "test",
	}

	notif := FormatMCPNotification(alert)

	if notif.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %q", notif.JSONRPC)
	}
	if notif.Method != "notifications/message" {
		t.Fatalf("expected method=notifications/message, got %q", notif.Method)
	}
	if notif.Params.Level != "error" {
		t.Fatalf("expected level=error, got %q", notif.Params.Level)
	}
	if notif.Params.Logger != NotificationLoggerName {
		t.Fatalf("expected logger=%s, got %q", NotificationLoggerName, notif.Params.Logger)
	}

	data := notif.Params.Data.(map[string]any)
	if data["title"] != "test failure" {
		t.Fatalf("expected title=test failure, got %v", data["title"])
	}
}

// ============================================
// EmitAlert Tests
// ============================================

func TestEmitAlert_Disabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = false

	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "test"})

	if buf.Len() != 0 {
		t.Fatal("EmitAlert should not write when disabled")
	}
}

func TestEmitAlert_SeverityFilter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "error"
	s.Config.Events = []string{"all"}

	// Info alert should be filtered out (below error threshold)
	s.EmitAlert(types.Alert{Severity: "info", Category: "regression", Title: "low severity"})
	if buf.Len() != 0 {
		t.Fatal("info alert should be filtered when severity_min=error")
	}

	// Error alert should pass
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "high severity"})
	if buf.Len() == 0 {
		t.Fatal("error alert should pass when severity_min=error")
	}
}

func TestEmitAlert_CategoryFilter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"ci"} // Only CI events

	// "regression" category should not match "ci" event filter
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "regression alert"})
	if buf.Len() != 0 {
		t.Fatal("regression should not match 'ci' event filter")
	}
}

func TestEmitAlert_Dedup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}

	alert := types.Alert{Severity: "error", Category: "regression", Title: "dup alert"}

	s.EmitAlert(alert)
	first := buf.Len()
	if first == 0 {
		t.Fatal("first EmitAlert should write")
	}

	// Same alert again should be deduped
	s.EmitAlert(alert)
	if buf.Len() != first {
		t.Fatal("duplicate alert within dedup window should be suppressed")
	}
}

func TestEmitAlert_WritesValidJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}

	s.EmitAlert(types.Alert{Severity: "warning", Category: "regression", Title: "test alert"})

	var notif MCPNotification
	if err := json.Unmarshal(buf.Bytes(), &notif); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, buf.String())
	}
	if notif.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", notif.JSONRPC)
	}
	if notif.Method != "notifications/message" {
		t.Errorf("method = %q, want notifications/message", notif.Method)
	}
}

func TestEmitAlert_Throttle(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}
	s.Config.ThrottleSeconds = 60 // very long throttle

	// First alert goes through
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "first"})
	first := buf.Len()
	if first == 0 {
		t.Fatal("first alert should be emitted")
	}

	// Second alert (different title to avoid dedup) should be throttled
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "second"})
	if buf.Len() != first {
		t.Fatal("second alert should be throttled")
	}

	// Should be in pending batch
	s.Mu.Lock()
	pending := len(s.PendingBatch)
	s.Mu.Unlock()
	if pending != 1 {
		t.Fatalf("expected 1 pending alert, got %d", pending)
	}
}

func TestEmitAlert_NilWriter(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	s.Writer = nil
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}

	// Should not panic with nil writer
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "test"})
}

func TestCheckRateReset(t *testing.T) {
	t.Parallel()

	s := NewStreamState()
	s.MinuteStart = time.Now().Add(-2 * time.Minute)
	s.NotifyCount = 50

	s.CheckRateReset(time.Now())

	s.Mu.Lock()
	count := s.NotifyCount
	s.Mu.Unlock()

	if count != 0 {
		t.Fatalf("CheckRateReset should reset count after minute boundary, got %d", count)
	}
}

func TestEmitAlert_RateLimit(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewStreamState()
	s.Writer = &buf
	s.Config.Enabled = true
	s.Config.SeverityMin = "info"
	s.Config.Events = []string{"all"}
	s.Config.ThrottleSeconds = 0 // no throttle

	// Exhaust the rate limit
	for i := 0; i < MaxNotificationsPerMinute; i++ {
		// Use unique titles to avoid dedup
		s.EmitAlert(types.Alert{
			Severity: "error",
			Category: "regression",
			Title:    strings.Repeat("x", i+1),
		})
	}

	prevLen := buf.Len()

	// Next one should be rate-limited
	s.EmitAlert(types.Alert{Severity: "error", Category: "regression", Title: "rate limited"})
	if buf.Len() != prevLen {
		t.Fatal("alert beyond rate limit should be suppressed")
	}
}
