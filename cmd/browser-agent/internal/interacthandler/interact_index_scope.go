// Purpose: Maintains a per-client/tab element index registry that maps numeric indices to CSS selectors for interact actions.
// Why: Enables stable element references across list_interactive snapshots with generation-based staleness detection.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type elementIndexScope struct {
	clientID string
	tabID    int
}

type elementIndexSnapshot struct {
	generation string
	selectors  map[int]string
	updatedAt  time.Time
}

type elementIndexRegistry struct {
	mu      sync.RWMutex
	byScope map[elementIndexScope]elementIndexSnapshot
}

func newElementIndexRegistry() *elementIndexRegistry {
	return &elementIndexRegistry{
		byScope: make(map[elementIndexScope]elementIndexSnapshot),
	}
}

func normalizeElementIndexClientID(clientID string) string {
	trimmed := strings.TrimSpace(clientID)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func makeElementIndexScope(clientID string, tabID int) elementIndexScope {
	return elementIndexScope{
		clientID: normalizeElementIndexClientID(clientID),
		tabID:    tabID,
	}
}

func (r *elementIndexRegistry) store(clientID string, tabID int, generation string, selectors map[int]string) string {
	if r == nil {
		return ""
	}
	if generation == "" {
		generation = fmt.Sprintf("idx_%d", time.Now().UnixNano())
	}
	scope := makeElementIndexScope(clientID, tabID)

	cloned := make(map[int]string, len(selectors))
	for index, selector := range selectors {
		cloned[index] = selector
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byScope[scope] = elementIndexSnapshot{
		generation: generation,
		selectors:  cloned,
		updatedAt:  time.Now(),
	}
	return generation
}

func (r *elementIndexRegistry) resolve(clientID string, tabID int, index int, generation string) (string, bool, bool, string) {
	if r == nil {
		return "", false, false, ""
	}
	scope := makeElementIndexScope(clientID, tabID)

	r.mu.RLock()
	snapshot, ok := r.byScope[scope]
	r.mu.RUnlock()
	if !ok {
		return "", false, false, ""
	}
	if generation != "" && snapshot.generation != generation {
		return "", false, true, snapshot.generation
	}
	selector, found := snapshot.selectors[index]
	return selector, found, false, snapshot.generation
}
