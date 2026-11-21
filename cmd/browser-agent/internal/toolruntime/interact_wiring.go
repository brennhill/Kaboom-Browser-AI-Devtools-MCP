// interact_wiring.go — Composes the interact action owners from the browser-command owner and capture.
// Why: interact families take narrow ports (toolinteract.Commands, WaitCommand, tracking state)
// injected here at the composition root instead of reaching through the whole Capture.
// Docs: docs/features/feature/interact-explore/index.md

package toolruntime

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactbatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/a11yaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
	observepage "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/page"
)

func initializeInteractActionOwners(h *ToolHandler) {
	if h.Guards == nil {
		h.Guards = toolguard.New(extensionOf(h), h.shutdownCtx, defaultExtensionReadinessTimeout())
	}
	var (
		interactCommands toolinteract.Commands
		waitCommand      func(string, time.Duration) (*queries.CommandResult, bool)
		pageTab          toolinteract.TrackedTab
		browserTab       toolinteract.BrowserTab
		perfBaseline     toolinteract.PerfBaseline
	)
	trackingStatus := func() (bool, int, string) { return false, 0, "" }
	vitals := func() []performance.PerformanceSnapshot { return nil }
	if h.capture != nil {
		interactCommands = h.capture.Commands()
		waitCommand = h.capture.Commands().WaitCommand
		pageTab, browserTab = h.capture.Extension(), h.capture.Extension()
		perfBaseline = h.capture.Performance()
		trackingStatus = h.capture.Extension().GetTrackingStatus
		vitals = h.capture.Performance().Entries
	}
	h.interactRuntime = toolinteract.NewActionRuntime(toolinteract.RuntimeDeps{
		RequireCSPClear:     h.Guards.RequireCSPClear,
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery, MaybeWaitForCommand: h.asyncCommands.MaybeWaitForCommand,
		RecordAIAction: h.actionRecorder.Record,
		DefaultEvidenceCapture: func(clientID string) toolinteract.EvidenceShot {
			return toolinteract.CaptureEvidence(interactCommands, trackingEnabled(h), clientID)
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
		RequireTabTracking: h.Guards.RequireTabTracking, Tab: pageTab, Vitals: vitals, Commands: interactCommands,
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
		RequireTabTracking: h.Guards.RequireTabTracking, Tab: browserTab, Perf: perfBaseline, Commands: interactCommands,
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
		buildWorkflowDeps(h, trackingStatus),
	)
	h.batchActions = interactbatch.New(interactbatch.Deps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension, WaitCommand: waitCommand,
		RecordAIAction: h.actionRecorder.Record, Interact: h.toolInteract, ReplayMu: &h.replayMu,
	})
}

// buildWorkflowDeps wires workflow actions to typed use cases (a11y audit,
// SARIF export) and tracked-tab state rather than MCP-shaped tool handlers.
func buildWorkflowDeps(h *ToolHandler, trackingStatus func() (bool, int, string)) toolinteract.WorkflowDeps {
	return toolinteract.WorkflowDeps{
		TrackingStatus: trackingStatus,
		AuditA11y: func(scope string) (a11yaudit.Result, error) {
			return newA11yService(h).Audit(a11yaudit.Options{Scope: scope})
		},
		A11yFailure: func(req mcp.JSONRPCRequest, err error) mcp.JSONRPCResponse {
			return observepage.A11yFailure(req, err, h.Guards.DiagnosticHintString())
		},
		ExportSARIF: func(a11yResult json.RawMessage, scope, saveTo string) (map[string]any, error) {
			return toolgenerate.BuildSARIF(h.state.Version, a11yResult, scope, saveTo, false)
		},
		Now: time.Now,
		WaitForTrackedURLChange: func(beforeURL string, timeout time.Duration) (string, bool) {
			return h.capture.Extension().WaitForTrackedURLChange(h.shutdownCtx, beforeURL, timeout)
		},
	}
}
