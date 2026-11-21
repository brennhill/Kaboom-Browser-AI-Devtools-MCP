// interact_wiring.go — Composes the interact action owners from the browser-command owner and capture.
// Why: interact families take narrow ports (toolinteract.Commands, WaitCommand, tracking state)
// injected here at the composition root instead of reaching through the whole Capture.
// Docs: docs/features/feature/interact-explore/index.md

package toolruntime

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactbatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
)

func initializeInteractActionOwners(h *ToolHandler) {
	if h.Guards == nil {
		h.Guards = toolguard.New(h.capture, h.shutdownCtx, defaultExtensionReadinessTimeout())
	}
	captureStore := func() *capture.Capture { return h.capture }
	var interactCommands toolinteract.Commands
	var waitCommand func(string, time.Duration) (*queries.CommandResult, bool)
	trackingEnabled := func() bool { return false }
	if h.capture != nil {
		interactCommands = h.capture.Commands()
		waitCommand = h.capture.Commands().WaitCommand
		trackingEnabled = func() bool {
			enabled, _, _ := h.capture.Extension().GetTrackingStatus()
			return enabled
		}
	}
	h.interactRuntime = toolinteract.NewActionRuntime(toolinteract.RuntimeDeps{
		RequireCSPClear:     h.Guards.RequireCSPClear,
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery, MaybeWaitForCommand: h.asyncCommands.MaybeWaitForCommand,
		RecordAIAction: h.actionRecorder.Record,
		DefaultEvidenceCapture: func(clientID string) toolinteract.EvidenceShot {
			return toolinteract.CaptureEvidence(interactCommands, trackingEnabled, clientID)
		},
	})
	h.domActions = toolinteract.NewDOMActions(h.interactRuntime, toolinteract.DOMDeps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension,
		RequireTabTracking: h.Guards.RequireTabTracking, RecordDOMPrimitiveAction: h.actionRecorder.RecordDOMPrimitive,
	})
	h.storageActions = toolinteract.NewStorageActions(h.interactRuntime, toolinteract.StorageDeps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension,
		RequireTabTracking: h.Guards.RequireTabTracking,
	})
	h.pageActions = toolinteract.NewPageActions(h.interactRuntime, h.domActions, h.storageActions, toolinteract.PageDeps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension,
		RequireTabTracking: h.Guards.RequireTabTracking, Capture: captureStore, Commands: interactCommands,
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery, RecordAIAction: h.actionRecorder.Record,
		MarkDrawStarted: func() {
			if h.annotationStore != nil {
				h.annotationStore.MarkDrawStarted()
			}
		},
		CaptureScreenshot: newScreenshotService(h).Capture,
		Diagnostics:       h.stateRecovery,
		PageInfo:          newPageInfoService(h).Read,
	})
	h.browserActions = toolinteract.NewBrowserActions(h.interactRuntime, h.pageActions, toolinteract.BrowserDeps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension,
		RequireTabTracking: h.Guards.RequireTabTracking, Capture: captureStore, Commands: interactCommands,
		InjectCSPBlockedActions: h.Guards.InjectCSPBlockedActions,
		GetListenPort: func() int {
			if h.state.ListenPort != nil {
				return h.state.ListenPort.Get()
			}
			return serverdefaults.Port
		},
	})
	h.workflowActions = toolinteract.NewWorkflowActions(
		h.interactRuntime, h.domActions, h.browserActions, h.pageActions,
		toolinteract.WorkflowDeps{
			Capture: captureStore, ToolAnalyze: h.analyzeDispatcher.Handle,
			ToolExportSARIF: h.generateDispatcher.ExportSARIF, Now: time.Now,
			WaitForTrackedURLChange: func(beforeURL string, timeout time.Duration) (string, bool) {
				return captureStore().Extension().WaitForTrackedURLChange(h.shutdownCtx, beforeURL, timeout)
			},
		},
	)
	h.batchActions = interactbatch.New(interactbatch.Deps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension, WaitCommand: waitCommand,
		RecordAIAction: h.actionRecorder.Record, Interact: h.toolInteract, ReplayMu: &h.replayMu,
	})
}
