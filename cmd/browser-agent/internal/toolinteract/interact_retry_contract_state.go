// Purpose: Stores and retrieves retry-contract state keyed by command correlation id.
// Why: Centralizes locking and lifecycle rules for retry context across interact command results.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func (h *InteractActionHandler) armRetryContract(correlationID, action string, args json.RawMessage) {
	if h == nil || correlationID == "" {
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}
	action = strings.ToLower(strings.TrimSpace(action))
	strategy, fingerprint := deriveRetryStrategy(action, args)
	parentCorrID := parseRetryParentCorrelationID(args)

	state := &commandRetryState{
		Attempt:             1,
		MaxAttempts:         maxRetryAttemptsPerStep,
		Action:              action,
		Strategy:            strategy,
		StrategyFingerprint: fingerprint,
		ChangedStrategy:     true,
		ParentCorrelationID: parentCorrID,
		CreatedAt:           time.Now(),
	}

	if parentCorrID != "" {
		if parent, ok := h.getRetryState(parentCorrID); ok {
			state.Attempt = parent.Attempt + 1
			if state.Attempt > state.MaxAttempts {
				state.Attempt = state.MaxAttempts
				state.PolicyViolation = "attempt_limit_exceeded"
			}
			state.ChangedStrategy = state.StrategyFingerprint != parent.StrategyFingerprint
			if !state.ChangedStrategy {
				state.PolicyViolation = "strategy_unchanged"
			}
		} else {
			// Treat explicit parent chaining as retry attempt even if parent context has expired.
			state.Attempt = 2
			state.PolicyViolation = "parent_context_missing"
		}
	}

	h.storeRetryState(correlationID, state)
}

func (h *InteractActionHandler) getRetryState(correlationID string) (*commandRetryState, bool) {
	h.retryContractMu.Lock()
	defer h.retryContractMu.Unlock()
	state, ok := h.retryByCommand[correlationID]
	return state, ok
}

func (h *InteractActionHandler) storeRetryState(correlationID string, state *commandRetryState) {
	h.retryContractMu.Lock()
	defer h.retryContractMu.Unlock()

	if h.retryByCommand == nil {
		h.retryByCommand = make(map[string]*commandRetryState)
	}
	h.retryByCommand[correlationID] = state
	h.pruneRetryStatesLocked(2048)
}

// pruneRetryStatesLocked evicts oldest-first until the map holds at most
// maxEntries. The caller must hold retryContractMu.
//
// In production this runs on every storeRetryState, so the map only ever exceeds
// the cap by a single entry and one eviction would suffice — the previous
// implementation relied on that and removed exactly one. It now trims all the way
// to the target so it stays correct as a general-purpose helper (e.g. if the cap
// is lowered, or it is ever called after a batch insert).
func (h *InteractActionHandler) pruneRetryStatesLocked(maxEntries int) {
	excess := len(h.retryByCommand) - maxEntries
	if excess <= 0 {
		return
	}

	type keyedEntry struct {
		key     string
		created time.Time
	}
	entries := make([]keyedEntry, 0, len(h.retryByCommand))
	for key, st := range h.retryByCommand {
		entries = append(entries, keyedEntry{key: key, created: st.CreatedAt})
	}
	// Oldest first; break CreatedAt ties by key so eviction is deterministic
	// regardless of map iteration order.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].created.Equal(entries[j].created) {
			return entries[i].key < entries[j].key
		}
		return entries[i].created.Before(entries[j].created)
	})
	for i := 0; i < excess; i++ {
		delete(h.retryByCommand, entries[i].key)
	}
}
