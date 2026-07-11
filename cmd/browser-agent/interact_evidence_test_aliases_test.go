// Purpose: Test aliases for evidence types/functions that moved to internal/interacthandler.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/interacthandler"
)

// evidenceShot aliases the exported type from interacthandler.
type evidenceShot = interacthandler.EvidenceShot

// evidenceCaptureFn is a shim that mimics the old package-level var.
// Tests read/write this var; the setter syncs to interacthandler.SetEvidenceCaptureFn.
var evidenceCaptureFn func(*ToolHandler, string) evidenceShot

// syncEvidenceCaptureFn installs the current evidenceCaptureFn into interacthandler.
// Call after assigning evidenceCaptureFn in a test.
func syncEvidenceCaptureFn() {
	if evidenceCaptureFn == nil {
		interacthandler.ResetEvidenceCaptureFn()
		return
	}
	fn := evidenceCaptureFn
	interacthandler.SetEvidenceCaptureFn(func(_ *interacthandler.Deps, clientID string) interacthandler.EvidenceShot {
		return fn(nil, clientID)
	})
}
