// types.go — Snapshot, change and diff-result models plus the tracked header set.
// Purpose: Defines security diff domain models and manager configuration.
// Why: Keeps core diff data structures stable while behavior is split into focused files.
// Docs: docs/features/feature/security-hardening/index.md

// Package diff captures named snapshots of a site's security posture — headers,
// cookie flags, per-endpoint auth and transport scheme — and reports what
// regressed or improved between two of them.
//
// It shares the HTTP primitives in internal/security/httpsec with the scan
// package (IsHTMLResponse decides which responses contribute headers,
// ParseSingleCookie normalizes Set-Cookie) and depends on no other sibling.
package diff

import (
	"sync"
	"time"
)

// ============================================
// Types
// ============================================

// Manager stores and compares security posture snapshots.
//
// Invariants:
// - snapshots map and order slice are mutated only under mu.
// - order is insertion-ordered and used for deterministic oldest-first eviction.
// - maxSnaps and ttl define bounded in-memory retention.
//
// Failure semantics:
// - Invalid snapshot names/actions are rejected with explicit errors.
type Manager struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot
	order     []string // insertion order for LRU eviction
	maxSnaps  int
	ttl       time.Duration
	now       func() time.Time
}

// Snapshot captures normalized security posture at one instant.
//
// Invariants:
// - Header/cookie/auth/transport maps are keyed by normalized origin or endpoint strings.
type Snapshot struct {
	Name      string                       `json:"name"`
	TakenAt   time.Time                    `json:"taken_at"`
	Headers   map[string]map[string]string `json:"headers"`   // origin -> headerName -> value
	Cookies   map[string][]Cookie          `json:"cookies"`   // origin -> cookies
	Auth      map[string]bool              `json:"auth"`      // endpoint (method+url) -> has_auth
	Transport map[string]string            `json:"transport"` // origin -> "https" or "http"
}

// Cookie records cookie attributes for comparison.
type Cookie struct {
	Name     string `json:"name"`
	HttpOnly bool   `json:"httponly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"samesite"`
}

// Result is the comparison response.
type Result struct {
	Verdict      string   `json:"verdict"` // "regressed", "improved", "unchanged"
	Regressions  []Change `json:"regressions"`
	Improvements []Change `json:"improvements"`
	Summary      Summary  `json:"summary"`
}

// Change describes a single security posture change.
type Change struct {
	Category       string `json:"category"` // "headers", "cookies", "auth", "transport"
	Severity       string `json:"severity"` // "critical", "high", "warning", "info"
	Origin         string `json:"origin,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	Change         string `json:"change"` // "header_removed", "header_added", etc.
	Header         string `json:"header,omitempty"`
	CookieName     string `json:"cookie_name,omitempty"`
	Flag           string `json:"flag,omitempty"`
	Before         string `json:"before"`
	After          string `json:"after"`
	Recommendation string `json:"recommendation"`
}

// Summary provides aggregate change counts.
type Summary struct {
	TotalRegressions  int            `json:"total_regressions"`
	TotalImprovements int            `json:"total_improvements"`
	BySeverity        map[string]int `json:"by_severity"`
	ByCategory        map[string]int `json:"by_category"`
}

// SnapshotListEntry is a summary for the list response.
type SnapshotListEntry struct {
	Name    string `json:"name"`
	TakenAt string `json:"taken_at"`
	Age     string `json:"age"`
	Expired bool   `json:"expired"`
}

// ============================================
// Security headers tracked for diff comparison
// ============================================

var trackedHeaders = []string{
	"Strict-Transport-Security",
	"X-Content-Type-Options",
	"X-Frame-Options",
	"Content-Security-Policy",
	"Referrer-Policy",
	"Permissions-Policy",
}

var headerRemovedRecommendations = map[string]string{
	"X-Frame-Options":           "X-Frame-Options was present before but is now missing. This exposes the app to clickjacking.",
	"Strict-Transport-Security": "Strict-Transport-Security was present before but is now missing. This exposes the app to MITM downgrade.",
	"X-Content-Type-Options":    "X-Content-Type-Options was present before but is now missing. This exposes the app to MIME sniffing.",
	"Content-Security-Policy":   "Content-Security-Policy was present before but is now missing. This exposes the app to XSS.",
	"Referrer-Policy":           "Referrer-Policy was present before but is now missing. This exposes the app to referrer leakage.",
	"Permissions-Policy":        "Permissions-Policy was present before but is now missing. This exposes the app to feature abuse.",
}

// ============================================
// Constructor
// ============================================

// NewManager creates a bounded snapshot store.
//
// Invariants:
// - Defaults favor short-lived regression checks (maxSnaps=5, ttl=4h).
func NewManager() *Manager {
	return &Manager{
		snapshots: make(map[string]*Snapshot),
		order:     make([]string, 0),
		maxSnaps:  5,
		ttl:       4 * time.Hour,
		now:       time.Now,
	}
}
