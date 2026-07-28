// Purpose: Declares diff result structs: Result, ErrorDiff, NetworkDiff, PerformanceDiff, Summary.
// Docs: docs/features/feature/request-session-correlation/index.md

// types.go — Diff computation types.

// Package snapdiff computes the difference between two named browser-state
// snapshots: which console errors appeared or were resolved, which endpoints
// changed status, which performance metrics regressed, and the overall verdict.
//
// It is a pure function of its two inputs. Nothing here reads or writes
// SessionManager state, so snapdiff is a leaf: it depends only on the canonical
// snapshot contract in internal/types (plus internal/capture for URL path
// extraction and internal/performance for the metric shape).
package snapdiff

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Result is the full comparison result between two snapshots.
type Result struct {
	A           string          `json:"a"`
	B           string          `json:"b"`
	Errors      ErrorDiff       `json:"errors"`
	Network     NetworkDiff     `json:"network"`
	Performance PerformanceDiff `json:"performance"`
	Summary     Summary         `json:"summary"`
}

// ErrorDiff holds the error comparison between two snapshots.
type ErrorDiff struct {
	New       []types.SnapshotError `json:"new"`
	Resolved  []types.SnapshotError `json:"resolved"`
	Unchanged []types.SnapshotError `json:"unchanged"`
}

// NetworkDiff holds the network comparison between two snapshots.
type NetworkDiff struct {
	NewErrors        []types.SnapshotNetworkRequest `json:"new_errors"`
	StatusChanges    []NetworkChange                `json:"status_changes"`
	NewEndpoints     []types.SnapshotNetworkRequest `json:"new_endpoints"`
	MissingEndpoints []types.SnapshotNetworkRequest `json:"missing_endpoints"`
}

// NetworkChange represents a status code change for the same endpoint.
type NetworkChange struct {
	Method         string `json:"method"`
	URL            string `json:"url"`
	BeforeStatus   int    `json:"before"`
	AfterStatus    int    `json:"after"`
	DurationChange string `json:"duration_change,omitempty"`
}

// PerformanceDiff holds performance metric comparisons.
type PerformanceDiff struct {
	LoadTime     *MetricChange `json:"load_time,omitempty"`
	RequestCount *MetricChange `json:"request_count,omitempty"`
	TransferSize *MetricChange `json:"transfer_size,omitempty"`
}

// MetricChange holds before/after values for a numeric metric.
type MetricChange struct {
	Before     float64 `json:"before"`
	After      float64 `json:"after"`
	Change     string  `json:"change"`
	Regression bool    `json:"regression"`
}

// Summary holds aggregate comparison stats and verdict.
type Summary struct {
	Verdict                string `json:"verdict"`
	NewErrors              int    `json:"new_errors"`
	ResolvedErrors         int    `json:"resolved_errors"`
	PerformanceRegressions int    `json:"performance_regressions"`
	NewNetworkErrors       int    `json:"new_network_errors"`
}

// perfRegressionRatio is the increase factor above which a metric counts as a
// regression (>50% increase).
const perfRegressionRatio = 1.5
