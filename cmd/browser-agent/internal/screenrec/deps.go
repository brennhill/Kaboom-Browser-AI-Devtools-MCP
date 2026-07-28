// deps.go — the contract between screen recording and its host (package main).
// Why: the recording state machine and its filesystem layout are self-contained;
// what screenrec cannot own are the MCP gates (pilot/extension), the extension
// command queue and the action journal, all of which live on the host's
// ToolHandler. Those arrive as function fields, so every decision here is
// testable with fakes and the dependency arrow stays one-way (host -> screenrec).
//
// This is the recording action family's explicit host boundary. It is a struct,
// not an interface: the host's gates are unexported
// methods, and an interface would force ToolHandler to grow exported adapters for
// seams it already hands to two other sub-packages as plain funcs.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Deps holds everything the recording handlers need from the host.
type Deps struct {
	// EnqueuePendingQuery queues a command for the extension.
	EnqueuePendingQuery func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool)

	// RequirePilot gates on AI Web Pilot being enabled.
	RequirePilot toolguard.Check

	// RequireExtension gates on the browser extension being connected.
	RequireExtension toolguard.Check

	// RecordAIAction journals an AI-driven action to the enhanced actions buffer.
	RecordAIAction func(action, url string, extra map[string]any)

	// DiagnosticHint returns a StructuredError option carrying connection diagnostics.
	DiagnosticHint func() func(*mcp.StructuredError)

	// GetCommandResult retrieves a queued command's result by correlation ID.
	GetCommandResult func(correlationID string) (*queries.CommandResult, bool)
}

// InteractHandler owns the interact(screen_recording_start/screen_recording_stop)
// state machine.
type InteractHandler struct {
	deps Deps

	// Interact recording state gate (start/stop sequencing).
	// Exclusively owned by InteractHandler — never read or written by the host.
	recordInteractMu sync.Mutex
	recordInteract   State
}

// NewInteractHandler builds the recording sub-handler from host-owned seams.
func NewInteractHandler(deps Deps) *InteractHandler {
	return &InteractHandler{deps: deps}
}
