// handler.go — the generate(test_*) sub-handler and its contract with the host.
// Why: test generation reads exactly two things from the host — the console log
// buffer and the capture store — and both contracts (mcp.LogBufferReader,
// mcp.CaptureProvider) were already exported from internal/mcp. That made this the
// one tools_*/testgen_* cluster that needed no new Deps design to leave package main.
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
