// store.go — Bounded origin/page accumulation for one daemon session.
// Purpose: Creates the CSP generator and records origin observations with bounded storage.
// Why: Separates origin accumulation from policy generation and tooling integration.
package csp

import "time"

// NewGenerator creates a fresh accumulator for one daemon session.
func NewGenerator() *Generator {
	return &Generator{
		origins: make(map[string]*OriginEntry),
		pages:   make(map[string]bool),
		now:     time.Now,
	}
}

// RecordOrigin ingests one resource observation into bounded origin/page sets.
//
// Invariants:
// - Entry keys are stable "origin|resourceType" tuples.
// - FirstSeen is immutable per entry; LastSeen updates per observation.
//
// Failure semantics:
// - Capacity pressure evicts oldest origin entries; ingestion remains non-blocking.
func (g *Generator) RecordOrigin(origin, resourceType, pageURL string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := origin + "|" + resourceType
	now := g.now()

	entry, exists := g.origins[key]
	if !exists {
		entry = &OriginEntry{
			Origin:       origin,
			ResourceType: resourceType,
			Pages:        make(map[string]bool),
			Count:        0,
			FirstSeen:    now,
		}
		g.origins[key] = entry
	}

	entry.Count++
	entry.LastSeen = now
	if len(entry.Pages) < 1000 {
		entry.Pages[pageURL] = true
	}

	if len(g.pages) < 1000 {
		g.pages[pageURL] = true
	}

	if len(g.origins) > 10000 {
		g.evictOldestOrigin()
	}
}

// evictOldestOrigin removes the earliest-observed entry to enforce map bounds.
//
// Failure semantics:
// - If map is empty, operation is a no-op.
func (g *Generator) evictOldestOrigin() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range g.origins {
		if oldestKey == "" || v.FirstSeen.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.FirstSeen
		}
	}
	if oldestKey != "" {
		delete(g.origins, oldestKey)
	}
}

// Reset clears accumulated observations.
//
// Invariants:
// - Both origins and pages maps are replaced atomically under lock.
func (g *Generator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.origins = make(map[string]*OriginEntry)
	g.pages = make(map[string]bool)
}

// GetPages returns a detached list of observed page URLs.
//
// Failure semantics:
// - Ordering is map-iteration order (non-deterministic); callers must sort if needed.
func (g *Generator) GetPages() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pages := make([]string, 0, len(g.pages))
	for p := range g.pages {
		pages = append(pages, p)
	}
	return pages
}
