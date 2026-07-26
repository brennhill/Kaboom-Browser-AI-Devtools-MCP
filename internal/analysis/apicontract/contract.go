// Purpose: Defines API contract validator state and shared contract result types.
// Why: Keeps core model definitions stable while behavior is split into focused files.
// Docs: docs/features/feature/api-schema/index.md

// Package apicontract learns per-endpoint response baselines and detects drift against them.
//
// APIContractValidator normalizes endpoints (dynamic IDs collapsed to placeholders),
// learns a response shape once an endpoint has been seen enough times, then reports
// violations — missing/new fields, field and top-level type changes, and error-rate
// spikes — through analyze and report result types consumed by the analyze MCP tool.
package apicontract

import (
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	maxContractEndpoints     = 30 // Max tracked endpoints
	maxStatusHistory         = 20 // Status codes to keep per endpoint
	minCallsToEstablishShape = 3  // Observations needed before flagging violations
	maxViolationsPerEndpoint = 10 // Cap violations per endpoint
	maxShapeComparisonDepth  = 3  // Nested object comparison depth
)

// ============================================
// Types
// ============================================

// APIContractValidator tracks API response shapes and detects contract violations.
type APIContractValidator struct {
	mu       sync.RWMutex
	trackers map[string]*EndpointTracker
}

// EndpointTracker maintains the established shape for an endpoint.
type EndpointTracker struct {
	Endpoint         string                 `json:"endpoint"`          // "METHOD /path"
	EstablishedShape any                    `json:"established_shape"` // Learned response shape
	CallCount        int                    `json:"call_count"`
	SuccessCount     int                    `json:"success_count"`    // 2xx responses
	ConsistentCount  int                    `json:"consistent_count"` // Responses matching established shape
	StatusHistory    []int                  `json:"status_history"`   // Last N status codes
	FieldPresence    map[string]int         `json:"field_presence"`   // field -> count of appearances
	FieldTypes       map[string]string      `json:"field_types"`      // field -> inferred type
	FirstCalled      time.Time              `json:"first_called"`     // When endpoint was first seen
	LastCalled       time.Time              `json:"last_called"`
	Violations       []APIContractViolation `json:"violations"`
}

// APIContractViolation represents a detected contract violation.
type APIContractViolation struct {
	Endpoint          string                `json:"endpoint"`
	Type              string                `json:"type"`           // "shape_change", "type_change", "error_spike", "new_field", "null_field"
	ViolationType     string                `json:"violation_type"` // Same as Type, explicit for LLM consumption
	Severity          string                `json:"severity"`       // "critical", "high", "medium", "low"
	Description       string                `json:"description"`
	AffectedCallCount int                   `json:"affected_call_count"`     // How many calls violated this rule
	FirstSeenAt       string                `json:"first_seen_at,omitempty"` // RFC3339 when violation first detected
	LastSeenAt        string                `json:"last_seen_at,omitempty"`  // RFC3339 when violation last detected
	ExpectedShape     map[string]any        `json:"expected_shape,omitempty"`
	ActualShape       map[string]any        `json:"actual_shape,omitempty"`
	MissingFields     []string              `json:"missing_fields,omitempty"`
	NewFields         []string              `json:"new_fields,omitempty"`
	Field             string                `json:"field,omitempty"`
	ExpectedType      string                `json:"expected_type,omitempty"`
	ActualType        string                `json:"actual_type,omitempty"`
	SampleValue       any                   `json:"sample_value,omitempty"`
	StatusHistory     []int                 `json:"status_history,omitempty"`
	LastErrorBody     map[string]any        `json:"last_error_body,omitempty"`
	Occurrences       *ViolationOccurrences `json:"occurrences,omitempty"`
}

// ViolationOccurrences tracks when a violation was first seen and how often.
type ViolationOccurrences struct {
	ExpectedCount  int       `json:"expected_count"`
	ViolationCount int       `json:"violation_count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastViolation  time.Time `json:"last_violation"`
}

// APIContractFilter controls which endpoints are analyzed.
type APIContractFilter struct {
	URLFilter       string   `json:"url,omitempty"`
	IgnoreEndpoints []string `json:"ignore_endpoints,omitempty"`
}

// AnalyzeSummary provides aggregate counts for LLM consumption.
type AnalyzeSummary struct {
	Violations     int `json:"violations"`
	Endpoints      int `json:"endpoints"`
	TotalRequests  int `json:"total_requests"`
	CleanEndpoints int `json:"clean_endpoints"`
}

// AppliedFilterEcho echoes back the filter parameters that were applied.
type AppliedFilterEcho struct {
	URL             string   `json:"url,omitempty"`
	IgnoreEndpoints []string `json:"ignore_endpoints,omitempty"`
}

// APIContractAnalyzeResult is the response from the analyze action.
type APIContractAnalyzeResult struct {
	Action                 string                 `json:"action"`
	AnalyzedAt             string                 `json:"analyzed_at"`
	DataWindowStartedAt    string                 `json:"data_window_started_at,omitempty"`
	AppliedFilter          *AppliedFilterEcho     `json:"applied_filter,omitempty"`
	Summary                *AnalyzeSummary        `json:"summary"` // Aggregate counts
	Violations             []APIContractViolation `json:"violations"`
	TrackedEndpoints       int                    `json:"tracked_endpoints"`
	TotalRequestsAnalyzed  int                    `json:"total_requests_analyzed"`
	CleanEndpoints         int                    `json:"clean_endpoints"`
	PossibleViolationTypes []string               `json:"possible_violation_types"`
	Hint                   string                 `json:"hint,omitempty"` // Helpful hint when no violations found
}

// APIContractReportResult is the response from the report action.
type APIContractReportResult struct {
	Action            string                   `json:"action"`
	AnalyzedAt        string                   `json:"analyzed_at"`
	AppliedFilter     *AppliedFilterEcho       `json:"applied_filter,omitempty"`
	Endpoints         []EndpointContractReport `json:"endpoints"`
	ConsistencyLevels map[string]string        `json:"consistency_levels"`
}

// EndpointContractReport summarizes a single endpoint's contract state.
type EndpointContractReport struct {
	Endpoint         string         `json:"endpoint"`
	Method           string         `json:"method"`
	CallCount        int            `json:"call_count"`
	StatusCodes      map[string]int `json:"status_codes"`
	EstablishedShape map[string]any `json:"established_shape,omitempty"`
	Consistency      string         `json:"consistency"`
	ConsistencyScore float64        `json:"consistency_score"` // Numeric 0-1
	FirstCalledAt    string         `json:"first_called_at"`
	LastCalledAt     string         `json:"last_called_at"`
}

// ============================================
// Constructor
// ============================================

// NewAPIContractValidator creates a new validator.
func NewAPIContractValidator() *APIContractValidator {
	return &APIContractValidator{
		trackers: make(map[string]*EndpointTracker),
	}
}
