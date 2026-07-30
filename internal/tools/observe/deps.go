// Purpose: Declares what the observe handlers need from the host server, and the bounds every mode applies.
// Why: Deps is the single seam between this package and *ToolHandler in cmd/browser-agent; the limit clamp
// sits with it because every mode that accepts a limit runs it through clampLimit before touching a buffer.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps names the canonical read owners and operations used by observe modes.
type Deps struct {
	Capture              *capture.Capture
	LogEntries           func() ([]types.LogEntry, []time.Time)
	LogTotalAdded        func() int64
	ExecuteA11yQuery     func(string, []string, any, bool) (json.RawMessage, error)
	IsConsoleNoise       func(types.LogEntry) bool
	DiagnosticHintString func() string
	// WaterfallRefreshTimeout bounds the on-demand extension query. Zero uses
	// the production default; tests provide a short deterministic budget.
	WaterfallRefreshTimeout time.Duration
}

// MaxObserveLimit caps the limit parameter to prevent oversized responses.
const MaxObserveLimit = 1000

// clampLimit applies default and max bounds to a limit parameter.
func clampLimit(limit, defaultVal int) int {
	if limit <= 0 {
		return defaultVal
	}
	if limit > MaxObserveLimit {
		return MaxObserveLimit
	}
	return limit
}
