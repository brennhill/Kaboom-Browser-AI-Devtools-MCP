// Purpose: Defines canonical alert and CI-result payload types emitted by server-side monitoring features.
// Why: Standardizes alert payload structure so observe/analyze consumers can process alerts consistently.
// Docs: docs/features/feature/push-alerts/index.md

package types

import "time"

// ============================================
// Immediate Alerts
// ============================================

// Alert represents a server-generated alert that piggybacks on observe responses.
// Typically created by monitoring incoming browser events and detecting errors, network failures, etc.
type Alert struct {
	Severity  string `json:"severity"`         // "info", "warning", "error"
	Category  string `json:"category"`         // "regression", "anomaly", "ci", "noise", "threshold"
	Title     string `json:"title"`            // Short summary
	Detail    string `json:"detail,omitempty"` // Longer explanation
	Timestamp string `json:"timestamp"`        // ISO 8601
	Source    string `json:"source"`           // What generated it
	Count     int    `json:"count,omitempty"`  // Deduplication count (>1 means repeated)
}

// ============================================
// CI/CD Integration
// ============================================

// CIResult stores a CI/CD webhook result.
type CIResult struct {
	Status     string      `json:"status"`      // "success", "failure", "error"
	Source     string      `json:"source"`      // "github-actions", "gitlab-ci", "custom"
	Ref        string      `json:"ref"`         // Branch ref
	Commit     string      `json:"commit"`      // Commit SHA
	Summary    string      `json:"summary"`     // Human-readable summary
	Failures   []CIFailure `json:"failures"`    // Failed tests
	URL        string      `json:"url"`         // Link to CI run
	DurationMs int         `json:"duration_ms"` // Build duration
	ReceivedAt time.Time   `json:"-"`           // When we received it
}

// CIFailure represents a single test failure in a CI result.
type CIFailure struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}
