// interact_state_handler.go — Defines the dedicated interact state snapshot sub-handler.
// Purpose: Narrows ToolHandler responsibilities by isolating state save/load/capture behavior.
// Docs: docs/features/feature/state-time-travel/index.md

package interacthandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

// StateInteractHandler handles state save/load/list/delete operations.
type StateInteractHandler struct {
	deps *Deps

	// Concrete session store injected at construction.
	sessionStoreImpl *persistence.SessionStore
}

// NewStateInteractHandler creates a new StateInteractHandler with the given dependencies.
func NewStateInteractHandler(deps *Deps, store *persistence.SessionStore) *StateInteractHandler {
	return &StateInteractHandler{
		deps:             deps,
		sessionStoreImpl: store,
	}
}
