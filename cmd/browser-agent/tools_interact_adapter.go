// tools_interact_adapter.go — Bridges the interacthandler package to the main ToolHandler.
// Purpose: Constructs interacthandler.Deps from *ToolHandler and provides accessor methods.
// Why: Keeps the interacthandler package decoupled from the main package's god object.

package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/interacthandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// buildInteractDeps constructs a interacthandler.Deps wired to the given ToolHandler.
func buildInteractDeps(h *ToolHandler) *interacthandler.Deps {
	return &interacthandler.Deps{
		// Gate checks
		RequirePilot:       h.requirePilot,
		RequireExtension:   h.requireExtension,
		RequireTabTracking: h.requireTabTracking,
		RequireCSPClear:    h.requireCSPClear,

		// Command dispatch
		EnqueuePendingQuery: h.EnqueuePendingQuery,
		MaybeWaitForCommand: h.MaybeWaitForCommand,

		// Capture store
		Capture: func() *capture.Store { return h.capture },

		// Recording
		RecordAIAction: h.recordAIAction,
		RecordAIEnhancedAction: func(action capture.EnhancedAction) {
			h.recordAIEnhancedAction(action)
		},
		RecordDOMPrimitiveAction: h.recordDOMPrimitiveAction,

		// Cross-tool dispatch
		ToolInteract:    h.toolInteract,
		ToolAnalyze:     h.toolAnalyze,
		ToolExportSARIF: h.toolExportSARIF,

		// Response enrichment
		EnrichNavigateResponse:  h.enrichNavigateResponse,
		InjectCSPBlockedActions: h.injectCSPBlockedActions,

		// Screenshot/observe proxies
		GetScreenshot: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return observe.GetScreenshot(h, req, args)
		},
		GetPageInfo: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return observe.GetPageInfo(h, req, args)
		},

		// Annotation store
		MarkDrawStarted: func() {
			if h.annotationStore != nil {
				h.annotationStore.MarkDrawStarted()
			}
		},

		// Server info
		GetListenPort: func() int {
			if h.server != nil {
				return h.server.getListenPort()
			}
			return defaultPort
		},

		// Evidence capture
		DefaultEvidenceCapture: func(clientID string) interacthandler.EvidenceShot {
			return defaultEvidenceCaptureImpl(h, clientID)
		},

		// Session store
		RequireSessionStore: h.requireSessionStore,
		DiagnosticHint:      h.diagnosticHint,
		GetRedactionEngine: func() interacthandler.RedactionEngine {
			return h.GetRedactionEngine()
		},
		GetCommandResult: func(correlationID string) (*queries.CommandResult, bool) {
			return h.capture.GetCommandResult(correlationID)
		},

		// Shared mutex for batch/replay serialization
		ReplayMu: &replayMu,
	}
}

// interactAction returns the interact action handler from the interacthandler package.
func (h *ToolHandler) interactAction() *interacthandler.InteractActionHandler {
	return h.interactActionHandler
}

// stateInteract returns the state interact handler from the interacthandler package.
func (h *ToolHandler) stateInteract() *interacthandler.StateInteractHandler {
	return h.stateInteractHandler
}

// defaultEvidenceCaptureImpl captures an evidence screenshot using the ToolHandler's capture store.
func defaultEvidenceCaptureImpl(h *ToolHandler, clientID string) interacthandler.EvidenceShot {
	if h == nil || h.capture == nil {
		return interacthandler.EvidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return interacthandler.EvidenceShot{Error: "no_tracked_tab"}
	}

	queryID, qerr := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: json.RawMessage(`{}`),
		},
		12*time.Second,
		clientID,
	)
	if qerr != nil {
		return interacthandler.EvidenceShot{Error: "queue_full: " + qerr.Error()}
	}

	raw, err := h.capture.WaitForResult(queryID, 12*time.Second)
	if err != nil {
		return interacthandler.EvidenceShot{Error: "screenshot_timeout: " + err.Error()}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return interacthandler.EvidenceShot{Error: "screenshot_parse_error: " + err.Error()}
	}

	if errMsg, ok := payload["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
		return interacthandler.EvidenceShot{Error: strings.TrimSpace(errMsg)}
	}

	path, _ := payload["path"].(string)
	filename, _ := payload["filename"].(string)
	path = strings.TrimSpace(path)
	filename = strings.TrimSpace(filename)
	if path == "" {
		return interacthandler.EvidenceShot{
			Filename: filename,
			Error:    "screenshot_missing_path",
		}
	}

	return interacthandler.EvidenceShot{
		Path:     path,
		Filename: filename,
	}
}
