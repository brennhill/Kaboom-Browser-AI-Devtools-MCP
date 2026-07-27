// Purpose: Declares the capture package's local wire/state types and cross-package interfaces.
// Why: Keeps every capture-owned type declaration in one place instead of a dozen prefix-named stubs.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// SecurityFlag represents a detected security issue detected from network waterfall analysis.
type SecurityFlag struct {
	Type      string    `json:"type"`      // "suspicious_tld", "non_standard_port", etc.
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Origin    string    `json:"origin"`    // The flagged origin
	Message   string    `json:"message"`   // Human-readable explanation
	Resource  string    `json:"resource"`  // Specific resource URL (optional)
	PageURL   string    `json:"page_url"`  // Page that loaded this resource
	Timestamp time.Time `json:"timestamp"` // When flagged
}

// PerformanceStore manages performance snapshots and baselines with LRU eviction.
type PerformanceStore struct {
	snapshots       map[string]performance.Snapshot
	snapshotOrder   []string
	baselines       map[string]performance.Baseline
	baselineOrder   []string
	beforeSnapshots map[string]performance.Snapshot // keyed by correlation_id, for perf_diff
}

// NetworkWaterfallBuffer groups network waterfall ring buffer fields.
// Protected by parent Capture.mu (no separate lock).
type NetworkWaterfallBuffer struct {
	entries  []NetworkWaterfallEntry // Ring buffer of PerformanceResourceTiming data
	capacity int                     // Configurable capacity (default DefaultNetworkWaterfallCapacity=1000)
}

// ExtensionLogBuffer groups extension log ring buffer fields.
// Protected by parent Capture.mu (no separate lock).
type ExtensionLogBuffer struct {
	logs []ExtensionLog // Ring buffer of extension internal logs (max MaxExtensionLogs=500)
}

// ClientRegistry defines the interface for managing connected MCP clients.
// Implemented by *session.ClientRegistry. Called by HTTP handlers.
// Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.
//
// Return types use any to avoid an import cycle (session imports capture):
//   - List() returns []session.ClientInfo
//   - Register() returns *session.ClientState
//   - Get() returns *session.ClientState (nil if not found)
type ClientRegistry interface {
	// Count returns the number of registered clients.
	Count() int
	// List returns all registered clients as []session.ClientInfo.
	List() any
	// Register creates or updates a client registration, returning *session.ClientState.
	Register(cwd string) any
	// Get returns a specific client by ID as *session.ClientState, or nil if not found.
	Get(id string) any
	// Unregister removes a client by ID and reports whether the client existed.
	Unregister(id string) bool
}
