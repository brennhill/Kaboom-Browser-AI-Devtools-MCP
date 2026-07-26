// registry.go — The scoped element index: numeric indices from list_interactive
// mapped back to CSS selectors, per (client, tab), with a generation stamp.
//
// Package elemindex owns that mapping and nothing else. It is the one piece of
// InteractActionHandler's state with no dependency on MCP, on Deps, or on the
// browser: a map, an RWMutex, and one invariant — a caller holding an index from
// generation A must never be silently served a selector from generation B, because
// the page has re-rendered underneath it and index 7 is now a different element.
// Keeping that invariant in a package whose entire surface is Store/Resolve is what
// makes it checkable.
package elemindex

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type scope struct {
	clientID string
	tabID    int
}

type snapshot struct {
	generation string
	selectors  map[int]string
	updatedAt  time.Time
}

// Registry holds the most recent index->selector snapshot for each (client, tab).
// The zero value is not usable; call New.
type Registry struct {
	mu      sync.RWMutex
	byScope map[scope]snapshot
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		byScope: make(map[scope]snapshot),
	}
}

func normalizeClientID(clientID string) string {
	trimmed := strings.TrimSpace(clientID)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func makeScope(clientID string, tabID int) scope {
	return scope{
		clientID: normalizeClientID(clientID),
		tabID:    tabID,
	}
}

// Store replaces the snapshot for (clientID, tabID) and returns the generation it
// was stamped with. An empty generation is replaced by a fresh monotonic one, so
// callers always get back something they can quote in a later Resolve.
func (r *Registry) Store(clientID string, tabID int, generation string, selectors map[int]string) string {
	if r == nil {
		return ""
	}
	if generation == "" {
		generation = fmt.Sprintf("idx_%d", time.Now().UnixNano())
	}
	scope := makeScope(clientID, tabID)

	cloned := make(map[int]string, len(selectors))
	for index, selector := range selectors {
		cloned[index] = selector
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byScope[scope] = snapshot{
		generation: generation,
		selectors:  cloned,
		updatedAt:  time.Now(),
	}
	return generation
}

// Resolve maps index back to a selector within (clientID, tabID).
//
// Returns (selector, found, staleGeneration, currentGeneration). When the caller
// quotes a non-empty generation that no longer matches the stored one, Resolve
// refuses to answer and reports staleGeneration=true with the current generation,
// so the caller can tell "no such index" apart from "your index is out of date".
func (r *Registry) Resolve(clientID string, tabID int, index int, generation string) (string, bool, bool, string) {
	if r == nil {
		return "", false, false, ""
	}
	scope := makeScope(clientID, tabID)

	r.mu.RLock()
	snap, ok := r.byScope[scope]
	r.mu.RUnlock()
	if !ok {
		return "", false, false, ""
	}
	if generation != "" && snap.generation != generation {
		return "", false, true, snap.generation
	}
	selector, found := snap.selectors[index]
	return selector, found, false, snap.generation
}

// FormatGenerationConflict renders the operator-facing message for a stale index.
func FormatGenerationConflict(expected, actual string) string {
	if expected == "" || actual == "" {
		return "Element index generation mismatch. Call list_interactive again and retry with the latest index_generation."
	}
	return fmt.Sprintf("Element index generation mismatch (expected %q, latest %q). Call list_interactive again and retry with the latest index_generation.", expected, actual)
}
