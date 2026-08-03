// Purpose: Declares the bounded alert buffer state and its capacity/window constants.
// Why: Keeps the producer-side buffer model separate from the notification stream it emits to.
// Docs: docs/features/feature/push-alerts/index.md

// Package alertbuf owns the server-side alert buffer: it accumulates alerts, CI
// results and error timestamps, post-processes them (dedup, correlation, priority
// sorting, formatting) and pushes qualifying alerts to a streaming.StreamState.
//
// The dependency arrow is one-way: alertbuf imports streaming, never the reverse.
package alertbuf

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Alert Constants
// ============================================

const (
	AlertBufferCap       = 50
	CIResultsCap         = 10
	CorrelationWindow    = 5 * time.Second
	AnomalyWindowSeconds = 60
	AnomalyBucketSeconds = 10
)

// ============================================
// AlertBuffer
// ============================================

// AlertBuffer owns the alert, CI, and anomaly state.
// Fields are exported for test access (internal package only).
type AlertBuffer struct {
	Mu         sync.Mutex
	Alerts     []types.Alert
	CIResults  []types.CIResult
	ErrorTimes []time.Time
	Stream     *streaming.StreamState
	Dropped    int64
	AlertTimes []time.Time
}

// NewAlertBuffer creates an AlertBuffer with a default StreamState.
func NewAlertBuffer() *AlertBuffer {
	return &AlertBuffer{
		Stream: streaming.NewStreamState(),
	}
}
