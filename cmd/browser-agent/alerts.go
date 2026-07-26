// Purpose: Adapts streaming alert/CI primitives into ToolHandler-facing alert management methods.
// Why: Keeps alert buffering/dedup logic centralized in streaming while preserving legacy cmd package call sites.
// Docs: docs/features/feature/push-alerts/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Type Aliases (backward compatibility)
// ============================================

// Alert is a type alias for the canonical alert type from types package
type Alert = types.Alert

// CIResult is a type alias for the canonical CI result type from types package
type CIResult = types.CIResult

// CIFailure is a type alias for the canonical CI failure type from types package
type CIFailure = types.CIFailure

// ============================================
// Constant Aliases
// ============================================

const (
	alertBufferCap       = alertbuf.AlertBufferCap
	ciResultsCap         = alertbuf.CIResultsCap
	correlationWindow    = alertbuf.CorrelationWindow
	anomalyWindowSeconds = alertbuf.AnomalyWindowSeconds
	anomalyBucketSeconds = alertbuf.AnomalyBucketSeconds
)

// ============================================
// Function Aliases
// ============================================

var (
	deduplicateAlerts    = alertbuf.DeduplicateAlerts
	correlateAlerts      = alertbuf.CorrelateAlerts
	canCorrelate         = alertbuf.CanCorrelate
	mergeAlerts          = alertbuf.MergeAlerts
	sortAlertsByPriority = alertbuf.SortAlertsByPriority
	severityRank         = streaming.SeverityRank
	formatAlertsBlock    = alertbuf.FormatAlertsBlock
	buildAlertSummary    = alertbuf.BuildAlertSummary
)

// ============================================
// ToolHandler Delegation
// ============================================

// drainAlerts delegates to the AlertBuffer.
func (h *ToolHandler) drainAlerts() []Alert {
	return h.alertBuffer.DrainAlerts()
}
