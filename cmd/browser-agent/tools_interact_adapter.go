// tools_interact_adapter.go — Bridges the toolinteract package to the main ToolHandler.
// Purpose: Constructs toolinteract.Deps from *ToolHandler and provides accessor methods.
// Why: Keeps the toolinteract package decoupled from the main package's god object.

package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactstate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// buildInteractDeps constructs a toolinteract.Deps wired to the given ToolHandler.
func buildInteractDeps(h *ToolHandler) *toolinteract.Deps {
	if h.Guards == nil {
		h.Guards = toolguard.New(h.capture, h.shutdownCtx, defaultExtensionReadinessTimeout())
	}
	return &toolinteract.Deps{
		// Gate checks
		RequirePilot:       h.Guards.RequirePilot,
		RequireExtension:   h.Guards.RequireExtension,
		RequireTabTracking: h.Guards.RequireTabTracking,
		RequireCSPClear:    h.Guards.RequireCSPClear,

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
		ToolAnalyze:     h.analyzeDispatcher.Handle,
		ToolExportSARIF: h.generateDispatcher.ExportSARIF,

		// Response enrichment
		EnrichNavigateResponse:  h.enrichNavigateResponse,
		InjectCSPBlockedActions: h.Guards.InjectCSPBlockedActions,

		// Screenshot/observe proxies
		GetScreenshot: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observe.GetScreenshot(h, req, args)
		},
		GetPageInfo: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observe.GetPageInfo(h, req, args)
		},

		// annotation.Annotation store
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
		DefaultEvidenceCapture: func(clientID string) toolinteract.EvidenceShot {
			return defaultEvidenceCaptureImpl(h, clientID)
		},

		// Session store
		RequireSessionStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			return sessionStoreGuard(h.sessionStoreImpl, req)
		},
		DiagnosticHint: h.Guards.DiagnosticHint,
		GetRedactionEngine: func() toolinteract.RedactionEngine {
			return h.GetRedactionEngine()
		},
		GetCommandResult: func(correlationID string) (*queries.CommandResult, bool) {
			return h.capture.GetCommandResult(correlationID)
		},

		// Shared mutex for batch/replay serialization
		ReplayMu: &replayMu,
	}
}

// interactAction returns the interact action handler from the toolinteract package.
func (h *ToolHandler) interactAction() *toolinteract.InteractActionHandler {
	return h.interactActionHandler
}

// stateInteract returns the state interact handler from the toolinteract package.
func (h *ToolHandler) stateInteract() *interactstate.Handler {
	return h.stateInteractHandler
}

// defaultEvidenceCaptureImpl captures an evidence screenshot using the ToolHandler's capture store.
func defaultEvidenceCaptureImpl(h *ToolHandler, clientID string) toolinteract.EvidenceShot {
	if h == nil || h.capture == nil {
		return toolinteract.EvidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return toolinteract.EvidenceShot{Error: "no_tracked_tab"}
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
		return toolinteract.EvidenceShot{Error: "queue_full: " + qerr.Error()}
	}

	raw, err := h.capture.WaitForResult(queryID, 12*time.Second)
	if err != nil {
		return toolinteract.EvidenceShot{Error: "screenshot_timeout: " + err.Error()}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return toolinteract.EvidenceShot{Error: "screenshot_parse_error: " + err.Error()}
	}

	if errMsg, ok := payload["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
		return toolinteract.EvidenceShot{Error: strings.TrimSpace(errMsg)}
	}

	path, _ := payload["path"].(string)
	filename, _ := payload["filename"].(string)
	path = strings.TrimSpace(path)
	filename = strings.TrimSpace(filename)
	if path == "" {
		return toolinteract.EvidenceShot{
			Filename: filename,
			Error:    "screenshot_missing_path",
		}
	}

	return toolinteract.EvidenceShot{
		Path:     path,
		Filename: filename,
	}
}
