// registry.go — The scoped element index: handles from list_interactive and from find,
// mapped back to the elements they name, per (client, tab), with a generation stamp.
//
// Package elemindex owns that mapping and nothing else. It is the one piece of
// DOMActions' state with no dependency on MCP, action-family dependencies, or the
// browser: a map, an RWMutex, and one invariant — a caller holding a handle from
// generation A must never be silently served a target from generation B, because
// the page has re-rendered underneath it and index 7 is now a different element.
// Keeping that invariant in a package whose entire surface is Store/Resolve/ResolveRef
// is what makes it checkable.
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
	targets    map[int]Target
	updatedAt  time.Time
}

// Registry holds the most recent index->target snapshot for each (client, tab).
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
func (r *Registry) Store(clientID string, tabID int, generation string, targets map[int]Target) string {
	if r == nil {
		return ""
	}
	if generation == "" {
		generation = fmt.Sprintf("idx_%d", time.Now().UnixNano())
	}
	scope := makeScope(clientID, tabID)

	// Target is a value struct, so copying the map detaches the snapshot from the caller's
	// map entirely: a later mutation there cannot rewrite a stored target.
	cloned := make(map[int]Target, len(targets))
	for index, target := range targets {
		cloned[index] = target
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byScope[scope] = snapshot{
		generation: generation,
		targets:    cloned,
		updatedAt:  time.Now(),
	}
	return generation
}

// snapshotFor returns the stored snapshot for (clientID, tabID), if there is one.
func (r *Registry) snapshotFor(clientID string, tabID int) (snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.byScope[makeScope(clientID, tabID)]
	return snap, ok
}

// Resolve maps index back to a target within (clientID, tabID).
//
// Returns (target, found, staleGeneration, currentGeneration). When the caller
// quotes a non-empty generation that no longer matches the stored one, Resolve
// refuses to answer and reports staleGeneration=true with the current generation,
// so the caller can tell "no such index" apart from "your index is out of date".
func (r *Registry) Resolve(clientID string, tabID int, index int, generation string) (Target, bool, bool, string) {
	if r == nil {
		return Target{}, false, false, ""
	}
	snap, ok := r.snapshotFor(clientID, tabID)
	if !ok {
		return Target{}, false, false, ""
	}
	if generation != "" && snap.generation != generation {
		return Target{}, false, true, snap.generation
	}
	target, found := snap.targets[index]
	return target, found, false, snap.generation
}

// ResolveRef maps an accessibility handle ("ax_<backendNodeId>") back to a target within
// (clientID, tabID), under the SAME generation rule Resolve applies to numeric indices.
//
// That rule is the whole point of routing refs through here. Chrome reuses a backendNodeId
// once the node it named is destroyed, so a ref quoted after a re-render can name an
// entirely different control. Refusing it is the only way the caller learns the page moved.
func (r *Registry) ResolveRef(clientID string, tabID int, ref string, generation string) (Target, bool, bool, string) {
	if r == nil {
		return Target{}, false, false, ""
	}
	backendID, wellFormed := parseRef(ref)
	if !wellFormed {
		return Target{}, false, false, ""
	}
	snap, ok := r.snapshotFor(clientID, tabID)
	if !ok {
		return Target{}, false, false, ""
	}
	if generation != "" && snap.generation != generation {
		return Target{}, false, true, snap.generation
	}
	for _, target := range snap.targets {
		if target.AXBackendID == backendID {
			return target, true, false, snap.generation
		}
	}
	return Target{}, false, false, snap.generation
}

// FormatGenerationConflict renders the operator-facing message for a stale handle.
func FormatGenerationConflict(expected, actual string) string {
	if expected == "" || actual == "" {
		return "Element index generation mismatch. Call list_interactive or find again and retry with the latest index_generation."
	}
	return fmt.Sprintf("Element index generation mismatch (expected %q, latest %q). Call list_interactive or find again and retry with the latest index_generation.", expected, actual)
}
