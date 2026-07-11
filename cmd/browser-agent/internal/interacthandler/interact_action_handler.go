// interact_action_handler.go — Defines the dedicated interact action dispatch sub-handler.
// Purpose: Narrows ToolHandler responsibilities by isolating interact routing/jitter orchestration.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"math/rand/v2"
	"sync"
	"time"
)

// InteractActionHandler handles interact action dispatch, jitter, evidence, and retry contracts.
type InteractActionHandler struct {
	deps *Deps

	// Action jitter: randomized micro-delays before interact actions.
	jitterMu          sync.RWMutex
	actionJitterMaxMs int // max jitter before each interact action (0 = disabled)

	// Optional evidence capture state keyed by correlation_id.
	evidenceMu        sync.Mutex
	evidenceByCommand map[string]*commandEvidenceState

	// Deterministic retry contract metadata keyed by correlation_id.
	retryContractMu sync.Mutex
	retryByCommand  map[string]*commandRetryState

	// Scoped element index registry used by list_interactive/index follow-up actions.
	elementIndexRegistry *elementIndexRegistry
}

// NewInteractActionHandler creates a new InteractActionHandler with the given dependencies.
func NewInteractActionHandler(deps *Deps) *InteractActionHandler {
	return &InteractActionHandler{
		deps:                 deps,
		evidenceByCommand:    make(map[string]*commandEvidenceState),
		retryByCommand:       make(map[string]*commandRetryState),
		elementIndexRegistry: newElementIndexRegistry(),
	}
}

// SetJitter sets the maximum jitter delay in milliseconds.
func (h *InteractActionHandler) SetJitter(maxMs int) {
	h.jitterMu.Lock()
	defer h.jitterMu.Unlock()
	h.actionJitterMaxMs = maxMs
}

// GetJitter returns the current maximum jitter delay in milliseconds.
func (h *InteractActionHandler) GetJitter() int {
	h.jitterMu.RLock()
	defer h.jitterMu.RUnlock()
	return h.actionJitterMaxMs
}

// ReadOnlyInteractActions lists actions that should not have jitter applied.
var ReadOnlyInteractActions = map[string]bool{
	"list_interactive":          true,
	"get_text":                  true,
	"get_value":                 true,
	"get_attribute":             true,
	"query":                     true,
	"screenshot":                true,
	"list_states":               true,
	"state_list":                true,
	"get_readable":              true,
	"get_markdown":              true,
	"explore_page":              true,
	"run_a11y_and_export_sarif": true,
	"wait_for":                  true,
	"wait_for_stable":           true,
	"auto_dismiss_overlays":     true,
	"batch":                     true,
	"highlight":                 true,
	"subtitle":                  true,
	"clipboard_read":            true,
}

// ApplyJitter sleeps for a random duration up to maxMs if jitter is configured.
func (h *InteractActionHandler) ApplyJitter(action string) int {
	if ReadOnlyInteractActions[action] {
		return 0
	}
	h.jitterMu.RLock()
	maxMs := h.actionJitterMaxMs
	h.jitterMu.RUnlock()
	if maxMs <= 0 {
		return 0
	}
	jitterMs := 0
	if maxMs > 0 {
		jitterMs = rand.IntN(maxMs)
	}
	if jitterMs > 0 {
		time.Sleep(time.Duration(jitterMs) * time.Millisecond)
	}
	return jitterMs
}
