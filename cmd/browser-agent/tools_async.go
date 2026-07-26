// Purpose: Async command helpers for correlation IDs and lifecycle status normalization.

package main

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
)

// newCorrelationID delegates to internal/toolresp, the single implementation.
var newCorrelationID = toolresp.NewCorrelationID

func canonicalLifecycleStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return "queued"
	case "pending", "running", "still_processing":
		return "running"
	case "complete":
		return "complete"
	case "error":
		return "error"
	case "timeout", "expired":
		return "timeout"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return status
	}
}

var (
	asyncInitialWait  = 15 * time.Second
	asyncRetryWait    = 5 * time.Second
	asyncPollInterval = 500 * time.Millisecond
)
