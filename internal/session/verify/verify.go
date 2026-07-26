// Purpose: Defines verify_fix verification session types, manager state, and constructors.
// Why: Keeps core verification model centralized while action/snapshot logic is split into focused modules.
// Docs: docs/features/feature/request-session-correlation/index.md

// verify.go — Verification Loop (verify_fix) MCP tool.
// Provides before/after session comparison for fix verification.
// AI captures state before a fix, applies fix, captures after, compares to verify.
// Session lifecycle: start -> watch -> compare (or cancel at any point).

// Package verify implements the verify_fix loop: capture a baseline of browser
// state, watch while a fix is applied, then compare before/after to decide
// whether console and network errors were resolved, unchanged, or made worse.
//
// It shares exactly one thing with the snapshot manager next door — the
// session.CaptureStateReader interface it reads live state through — and keeps
// its own snapshot shape (VerifSnapshot), its own normalization, and its own
// verdict rules.
//
// NOTE: nothing constructs a VerificationManager outside this package's tests.
// verify_fix is not registered in the configure tool registry
// (cmd/browser-agent/tools_configure_registry.go), so the MCP surface it
// implements is currently unreachable at runtime. That predates this refactor
// and is left as-is here; it is reported separately rather than silently
// deleted or wired up.
package verify

import (
	"regexp"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// ============================================
// Constants
// ============================================

const (
	maxVerificationSessions   = 3
	defaultVerificationTTL    = 30 * time.Minute
	maxBaselineErrors         = 50
	maxBaselineNetworkEntries = 50
)

// ============================================
// Normalization Regex Patterns (verify-specific)
// ============================================

var (
	// UUID pattern for clustering across test runs.
	clusterUUIDRegex = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

	// ISO 8601 timestamps for clustering across test runs.
	clusterTimestampRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)

	// Numeric IDs for clustering across test runs.
	clusterNumericIDRegex = regexp.MustCompile(`\d{1,19}`)

	// File path with line number: filename.ext:123
	// More specific pattern than clustering.go uses.
	verifyFileLineRegex = regexp.MustCompile(`[\w./\\-]+\.[a-zA-Z]{1,4}:\d+`)
)

// ============================================
// Types
// ============================================

// VerificationSession tracks a before/after comparison.
type VerificationSession struct {
	ID        string    `json:"verif_session_id"`
	Label     string    `json:"label"`
	Status    string    `json:"status"` // "baseline_captured", "watching", "compared", "cancelled"
	URLFilter string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// Baseline snapshot (captured at "start")
	Baseline *VerifSnapshot `json:"baseline"`

	// Watch state
	WatchStartedAt *time.Time `json:"watch_started_at,omitempty"`

	// After snapshot (captured at "compare")
	After *VerifSnapshot `json:"after,omitempty"`
}

// VerifSnapshot is a point-in-time capture of browser state for verification.
type VerifSnapshot struct {
	CapturedAt         time.Time             `json:"captured_at"`
	ConsoleErrors      []VerifyError         `json:"console_errors"`
	NetworkErrors      []VerifyNetworkEntry  `json:"network_errors"`        // Only 4xx/5xx responses
	AllNetworkRequests []VerifyNetworkEntry  `json:"all_network,omitempty"` // All requests (for comparison)
	PageURL            string                `json:"page_url,omitempty"`
	Performance        *performance.Snapshot `json:"performance,omitempty"`
}

// VerifyError represents a console error in a verification snapshot.
type VerifyError struct {
	Message    string `json:"message"`
	Normalized string `json:"normalized"` // For matching
	Count      int    `json:"count"`
	Source     string `json:"source,omitempty"`
}

// VerifyNetworkEntry represents a network request in a verification snapshot.
type VerifyNetworkEntry struct {
	Method   string `json:"method"`
	URL      string `json:"url"`
	Path     string `json:"path"` // URL path only (for matching)
	Status   int    `json:"status"`
	Duration int    `json:"duration,omitempty"`
}

// ============================================
// Response Types
// ============================================

// StartResult is the response from the start action.
type StartResult struct {
	VerifSessionID string          `json:"verif_session_id"`
	Label          string          `json:"label,omitempty"`
	Status         string          `json:"status"`
	Baseline       BaselineSummary `json:"baseline"`
}

// BaselineSummary summarizes the captured baseline state.
type BaselineSummary struct {
	CapturedAt    time.Time     `json:"captured_at"`
	ConsoleErrors int           `json:"console_errors"`
	NetworkErrors int           `json:"network_errors"`
	ErrorDetails  []ErrorDetail `json:"error_details"`
}

// ErrorDetail describes a single error in the baseline.
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Count   int    `json:"count,omitempty"`
	Method  string `json:"method,omitempty"`
	URL     string `json:"url,omitempty"`
	Status  int    `json:"status,omitempty"`
}

// WatchResult is the response from the watch action.
type WatchResult struct {
	VerifSessionID string `json:"verif_session_id"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

// CompareResult is the response from the compare action.
type CompareResult struct {
	VerifSessionID string             `json:"verif_session_id"`
	Status         string             `json:"status"`
	Label          string             `json:"label,omitempty"`
	Result         VerificationResult `json:"result"`
}

// VerificationResult is the diff output.
type VerificationResult struct {
	Verdict         string          `json:"verdict"`
	Before          IssueSummary    `json:"before"`
	After           IssueSummary    `json:"after"`
	Changes         []VerifyChange  `json:"changes"`
	NewIssues       []VerifyChange  `json:"new_issues"`
	PerformanceDiff *VerifyPerfDiff `json:"performance_diff,omitempty"`
}

// IssueSummary provides aggregate error counts.
type IssueSummary struct {
	ConsoleErrors int `json:"console_errors"`
	NetworkErrors int `json:"network_errors"`
	TotalIssues   int `json:"total_issues"`
}

// VerifyChange describes a single change between before and after.
type VerifyChange struct {
	Type     string `json:"type"`     // "resolved", "new", "changed", "unchanged"
	Category string `json:"category"` // "console", "network"
	Before   string `json:"before"`
	After    string `json:"after"`
}

// VerifyPerfDiff holds performance change summary.
type VerifyPerfDiff struct {
	LoadTimeBefore string `json:"load_time_before,omitempty"`
	LoadTimeAfter  string `json:"load_time_after,omitempty"`
	Change         string `json:"change,omitempty"`
}

// CancelResult is the response from the cancel action.
type CancelResult struct {
	VerifSessionID string `json:"verif_session_id"`
	Status         string `json:"status"`
}

// StatusResult is the response from the status action.
type StatusResult struct {
	VerifSessionID string    `json:"verif_session_id"`
	Label          string    `json:"label,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// ============================================
// VerificationManager
// ============================================

// VerificationManager manages verification sessions.
type VerificationManager struct {
	mu       sync.RWMutex
	sessions map[string]*VerificationSession
	order    []string // Track insertion order
	reader   session.CaptureStateReader
	ttl      time.Duration
	idSeq    int
}

// NewVerificationManager creates a new VerificationManager.
func NewVerificationManager(reader session.CaptureStateReader) *VerificationManager {
	return &VerificationManager{
		sessions: make(map[string]*VerificationSession),
		order:    make([]string, 0),
		reader:   reader,
		ttl:      defaultVerificationTTL,
	}
}

// NewVerificationManagerWithTTL creates a new VerificationManager with custom TTL.
func NewVerificationManagerWithTTL(reader session.CaptureStateReader, ttl time.Duration) *VerificationManager {
	return &VerificationManager{
		sessions: make(map[string]*VerificationSession),
		order:    make([]string, 0),
		reader:   reader,
		ttl:      ttl,
	}
}
