// resetter.go — Coordinates destructive reset across canonical capture owners.
// Docs: docs/features/feature/buffer-clearing/index.md

package resetter

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
)

// TestBoundaryOwner clears active test-boundary state.
type TestBoundaryOwner interface{ ClearTestBoundaries() }

// Dependencies are the independently synchronized owners reset together.
type Dependencies struct {
	Extension     TestBoundaryOwner
	Telemetry     *telemetrystore.Store
	Performance   *perfstore.Store
	ExtensionLogs *logstore.Extension
}

// Resetter owns coordinated in-memory capture resets.
type Resetter struct{ deps Dependencies }

// New binds the coordinated reset to canonical state owners.
func New(deps Dependencies) *Resetter { return &Resetter{deps: deps} }

// ClearAll resets all capture-owned telemetry and returns cleared extension logs.
func (r *Resetter) ClearAll() int {
	r.deps.Extension.ClearTestBoundaries()
	r.deps.Telemetry.ClearAll()
	r.deps.Performance.Clear()
	return r.deps.ExtensionLogs.Clear()
}
