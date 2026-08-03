// Purpose: Defines streaming state/config model types for the MCP notification stream.
// Why: Keeps push-alert streaming contracts explicit across configuration and emission paths.
// Docs: docs/features/feature/push-alerts/index.md

package streaming

import (
	"io"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Stream Constants
// ============================================

const (
	DefaultThrottleSeconds    = 5
	DefaultSeverityMin        = "warning"
	MaxNotificationsPerMinute = 12
	DedupWindow               = 30 * time.Second
	MaxPendingBatch           = 100
	NotificationLoggerName    = identity.MCPServerName
)

// ============================================
// Stream Types
// ============================================

// StreamConfig holds the user-configured streaming settings.
type StreamConfig struct {
	Enabled         bool     `json:"enabled"`
	Events          []string `json:"events"`
	ThrottleSeconds int      `json:"throttle_seconds"`
	URLFilter       string   `json:"url"`
	SeverityMin     string   `json:"severity_min"`
}

// StreamState manages active context streaming.
type StreamState struct {
	Config       StreamConfig
	LastNotified time.Time
	SeenMessages map[string]time.Time // dedupKey → last sent
	NotifyCount  int                  // count in current minute
	MinuteStart  time.Time
	PendingBatch []types.Alert
	PendingSince time.Time
	DroppedCount int64
	Mu           sync.Mutex
	Writer       io.Writer // defaults to nil (no output)
}

// PressureSnapshot describes the bounded pending notification queue.
type PressureSnapshot struct {
	Size      int           `json:"size"`
	Capacity  int           `json:"capacity"`
	Dropped   int64         `json:"dropped_count"`
	OldestAge time.Duration `json:"oldest_age"`
	Saturated bool          `json:"saturated"`
}

// MCPNotification is the MCP notification format for streaming alerts.
type MCPNotification struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

// NotificationParams holds the notification payload.
type NotificationParams struct {
	Level  string `json:"level"`
	Logger string `json:"logger"`
	Data   any    `json:"data"`
}
