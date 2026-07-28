// handler.go — the generate(test_*) sub-handler and its contract with the host.
// Why: Test generation consumes only log, action, and network snapshots, so its
// composition boundary names those reads directly instead of exposing a host.
// Docs: docs/features/feature/test-generation/index.md

package testgenhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps supplies the three immutable snapshots consumed by test generation.
type Deps struct {
	LogEntries      func() []types.LogEntry
	EnhancedActions func() []types.EnhancedAction
	NetworkBodies   func() []types.NetworkBody
}

// Handler owns generate(what: test_from_context | test_heal | test_classify).
type Handler struct {
	deps Deps
}

// New builds the test-generation sub-handler.
func New(deps Deps) *Handler {
	return &Handler{deps: deps}
}
