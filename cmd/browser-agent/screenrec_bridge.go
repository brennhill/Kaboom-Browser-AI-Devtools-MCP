// screenrec_bridge.go — wires the screenrec sub-package into package main.
// Why: screenrec owns the tab/screen video recording feature end to end (state
// machine, on-disk layout, HTTP save/reveal). This file is the only thing main
// keeps: the Deps it builds, and the two MCP entry points whose `tool*` names are
// part of the observe/interact registries and must stay on ToolHandler (rule 17).
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
)

// screenrecDeps builds the screenrec seam set from this handler. Rebuilt per call
// site rather than cached so the method values are bound at call time, matching
// how daemonlifeDeps and interactDeps behave.
func (h *ToolHandler) screenrecDeps() screenrec.Deps {
	return screenrec.Deps{
		EnqueuePendingQuery: h.EnqueuePendingQuery,
		RequirePilot:        h.requirePilot,
		RequireExtension:    h.requireExtension,
		RecordAIAction:      h.recordAIAction,
		DiagnosticHint:      h.diagnosticHint,
		GetCommandResult:    h.getCommandResult,
	}
}

// toolObserveSavedVideos handles observe({what: "saved_videos"}).
// The listing is a pure filesystem scan, so it needs no handler state.
func (h *ToolHandler) toolObserveSavedVideos(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return screenrec.HandleObserveSavedVideos(req, args)
}
