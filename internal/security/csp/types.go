// types.go — Origin entry, generator state, params and response models.
// Purpose: Defines types for CSP origin entries, responses, and the CSP generator state.
// Why: Centralizes CSP type definitions so store, generate, and tooling modules share one schema.
// Docs: docs/features/feature/security-hardening/index.md

// Package csp accumulates observed resource origins and turns them into a
// Content-Security-Policy grounded in runtime traffic. The package owns its
// core models, bounded store, generation pipeline, directive helpers, and
// MCP-facing session overrides as one cohesive security feature.
package csp

import (
	"sync"
	"time"
)

// OriginEntry records aggregated observations for one origin/resource-type pair.
//
// Invariants:
// - Count is monotonic for a live entry.
// - Pages behaves as a bounded set (max 1000 page keys per entry).
type OriginEntry struct {
	Origin       string          `json:"origin"`
	ResourceType string          `json:"resource_type"`
	Pages        map[string]bool `json:"-"`
	Count        int             `json:"observation_count"`
	FirstSeen    time.Time       `json:"first_seen"`
	LastSeen     time.Time       `json:"last_seen"`
}

// Audit reports which whitelist overrides shaped a generated policy and where
// they came from, so a reviewer can distinguish session-only overrides from
// persistent configuration.
type Audit struct {
	SessionOverrides    []string `json:"session_overrides,omitempty"`
	PersistentWhitelist []string `json:"persistent_whitelist,omitempty"`
	OverrideSource      string   `json:"override_source,omitempty"`
}

// Generator maintains the origin accumulator and generates CSP policies.
//
// Invariants:
// - origins/pages maps are mutated only under mu.
// - origins map is bounded by eviction (max ~10k keys).
// - Generated policies are pure reads over accumulated state.
//
// Failure semantics:
// - Invalid/unknown resource types are excluded rather than causing generation failure.
type Generator struct {
	mu      sync.RWMutex
	origins map[string]*OriginEntry // key: "origin|resourceType"
	pages   map[string]bool         // all unique page URLs observed
	now     func() time.Time
}

// Params defines the input parameters for CSP generation.
type Params struct {
	Mode              string   `json:"mode"` // strict, moderate, report_only
	IncludeReportURI  bool     `json:"include_report_uri"`
	ExcludeOrigins    []string `json:"exclude_origins"`
	WhitelistOverride []string `json:"whitelist_override,omitempty"` // SESSION-ONLY temporary whitelist (not persisted)
	SuppressFlags     []string `json:"suppress_flags,omitempty"`     // SESSION-ONLY flag suppression (not persisted)
}

// Response is the full response from Generate.
type Response struct {
	CSPHeader           string              `json:"csp_header"`
	HeaderName          string              `json:"header_name"`
	MetaTag             string              `json:"meta_tag"`
	Directives          map[string][]string `json:"directives"`
	OriginDetails       []OriginDetail      `json:"origin_details"`
	FilteredOrigins     []FilteredOrigin    `json:"filtered_origins"`
	Observations        Observations        `json:"observations"`
	Warnings            []string            `json:"warnings"`
	RecommendedNextStep string              `json:"recommended_next_step"`
	Audit               *Audit              `json:"audit,omitempty"` // Security boundary audit info
}

// OriginDetail provides per-origin confidence and inclusion info.
type OriginDetail struct {
	Origin           string   `json:"origin"`
	Directive        string   `json:"directive"`
	Confidence       string   `json:"confidence"`
	ObservationCount int      `json:"observation_count"`
	FirstSeen        string   `json:"first_seen"`
	LastSeen         string   `json:"last_seen"`
	PagesSeenOn      []string `json:"pages_seen_on"`
	Included         bool     `json:"included"`
	ExclusionReason  string   `json:"exclusion_reason,omitempty"`
}

// FilteredOrigin describes an origin that was automatically filtered.
type FilteredOrigin struct {
	Origin string `json:"origin"`
	Reason string `json:"reason"`
}

// Observations summarizes the observation session.
type Observations struct {
	TotalResources  int `json:"total_resources"`
	UniqueOrigins   int `json:"unique_origins"`
	OriginsIncluded int `json:"origins_included"`
	OriginsFiltered int `json:"origins_filtered"`
	PagesVisited    int `json:"pages_visited"`
}

// resourceTypeToDirective maps resource types to CSP directive names.
var resourceTypeToDirective = map[string]string{
	"script":  "script-src",
	"style":   "style-src",
	"font":    "font-src",
	"img":     "img-src",
	"connect": "connect-src",
	"frame":   "frame-src",
	"media":   "media-src",
	"worker":  "worker-src",
}

// originProcessingResult holds intermediate state from processing origin entries.
type originProcessingResult struct {
	directives      map[string]map[string]bool
	originDetails   []OriginDetail
	filteredOrigins []FilteredOrigin
	totalResources  int
	uniqueOrigins   int
	originsIncluded int
	originsFiltered int
}
