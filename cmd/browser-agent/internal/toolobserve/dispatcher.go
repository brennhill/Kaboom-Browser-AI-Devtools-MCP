// dispatcher.go — Owns observe mode routing and server-side command observation.
// Docs: docs/features/feature/observe/index.md

package toolobserve

import (
	"encoding/json"
	"fmt"
	observelogs "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/logs"
	observenetwork "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/network"
	observepage "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/page"
	observesession "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/session"
	observetimeline "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/timeline"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	observecore "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const annotationCommandWaitTimeout = 55 * time.Second

type CommandStore interface {
	WaitForCommand(string, time.Duration) (*queries.CommandResult, bool)
	GetCommandResult(string) (*queries.CommandResult, bool)
	GetPendingCommands() []*queries.CommandResult
	GetCompletedCommands() []*queries.CommandResult
	GetFailedCommands() []*queries.CommandResult
}

type Config struct {
	Observe              observecore.Deps
	Local                Deps
	IsExtensionConnected func() bool
	Commands             CommandStore
	InProgress           func() []syncruntime.SyncInProgress
	AnnotationStore      *annotation.Store
	Annotations          toolrouting.Handler[observecore.Deps]
	AnnotationDetail     toolrouting.Handler[observecore.Deps]
	Recordings           toolrouting.Handler[observecore.Deps]
	RecordingActions     toolrouting.Handler[observecore.Deps]
	PlaybackResults      toolrouting.Handler[observecore.Deps]
	LogDiffReport        toolrouting.Handler[observecore.Deps]
	FormatCommand        func(mcp.JSONRPCRequest, queries.CommandResult, string) mcp.JSONRPCResponse
	InjectSummary        func(json.RawMessage) json.RawMessage
	DrainAlerts          func() []types.Alert
	DiagnosticHint       func(*mcp.StructuredError)
	StateDiagnostics     statediag.Reporter
}

type Dispatcher struct {
	observe  observecore.Deps
	commands CommandStore
	config   Config
	registry toolrouting.Registry[observecore.Deps]
}

func NewDispatcher(config Config) *Dispatcher {
	d := &Dispatcher{observe: config.Observe, commands: config.Commands, config: config}
	handlers := map[string]toolrouting.Handler[observecore.Deps]{
		"errors": wrap(observelogs.GetBrowserErrors), "logs": wrap(observelogs.GetBrowserLogs),
		"extension_logs": wrap(observelogs.GetExtensionLogs), "network_waterfall": wrap(observenetwork.GetNetworkWaterfall),
		"network_bodies": wrap(observenetwork.GetNetworkBodies), "websocket_events": wrap(observenetwork.GetWSEvents),
		"websocket_status": wrap(observenetwork.GetWSStatus), "actions": wrap(observesession.GetEnhancedActions),
		"vitals": wrap(observesession.GetWebVitals), "page": wrap(observepage.GetPageInfo), "tabs": wrap(observesession.GetTabs),
		"history": wrap(observesession.AnalyzeHistory), "pilot": wrap(observesession.ObservePilot),
		"timeline": wrap(observetimeline.GetSessionTimeline), "error_bundles": wrap(observetimeline.GetErrorBundles),
		"screenshot": wrap(observepage.GetScreenshot), "storage": wrap(observepage.GetStorage),
		"indexeddb": wrap(observepage.GetIndexedDB), "summarized_logs": wrap(observelogs.GetSummarizedLogs),
		"transients":  wrap(observesession.GetTransients),
		"annotations": config.Annotations, "annotation_detail": config.AnnotationDetail,
		"draw_history": d.drawHistory, "draw_session": d.drawSession,
		"page_inventory": local(config.Local, HandlePageInventory), "inbox": local(config.Local, HandleInbox), "site_menus": local(config.Local, HandleSiteMenus),
		"command_result": func(_ observecore.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.CommandResult(req, args)
		},
		"pending_commands": d.pendingCommands,
		"failed_commands": func(_ observecore.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.FailedCommands(req, args)
		},
		"saved_videos": d.savedVideos,
		"recordings":   config.Recordings, "recording_actions": config.RecordingActions,
		"playback_results": config.PlaybackResults, "log_diff_report": config.LogDiffReport,
	}
	d.registry = toolrouting.Registry[observecore.Deps]{
		Handlers: handlers,
		Resolution: toolrouting.Resolution{
			ToolName: "observe", ValidModes: strings.Join(util.SortedMapKeys(handlers), ", "),
		},
		PreDispatch: func(_ observecore.Deps, _ mcp.JSONRPCRequest, args json.RawMessage, _ string) (json.RawMessage, *mcp.JSONRPCResponse) {
			return config.InjectSummary(args), nil
		},
		PostDispatch: func(_ observecore.Deps, _ mcp.JSONRPCRequest, resp mcp.JSONRPCResponse, what string) mcp.JSONRPCResponse {
			if !config.IsExtensionConnected() && !ServerSideObserveModes[what] {
				resp = PrependDisconnectWarning(resp)
			}
			if alerts := config.DrainAlerts(); len(alerts) > 0 {
				resp = AppendAlertsToResponse(resp, alerts)
			}
			return resp
		},
	}
	return d
}

func wrap(fn func(observecore.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[observecore.Deps] {
	return func(h observecore.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func local(deps Deps, fn func(Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[observecore.Deps] {
	return func(_ observecore.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

func (d *Dispatcher) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return toolrouting.Dispatch(d.observe, req, args, d.registry)
}

func (d *Dispatcher) ValidModes() []string { return util.SortedMapKeys(d.registry.Handlers) }

func (d *Dispatcher) drawHistory(_ observecore.Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.ListDrawHistory(req, dir, err)
}

func (d *Dispatcher) drawSession(_ observecore.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.LoadDrawSession(d.config.AnnotationStore, req, args, dir, err)
}

func (d *Dispatcher) CommandResult(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	if d.commands == nil {
		return mcp.Fail(req, mcp.ErrNoData, "Command state is unavailable", "Connect the browser extension and retry.")
	}
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil && len(args) > 0 {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	if response, blocked := toolresp.RequireString(req, params.CorrelationID, "correlation_id", "Add the 'correlation_id' parameter and call again"); blocked {
		return response
	}
	var command *queries.CommandResult
	var found bool
	if strings.HasPrefix(params.CorrelationID, "ann_") {
		command, found = d.commands.WaitForCommand(params.CorrelationID, annotationCommandWaitTimeout)
		if !found {
			return mcp.Fail(req, mcp.ErrNoData, "Annotation command not found: "+params.CorrelationID,
				"The command may have expired (10 min TTL). Start a new draw mode session.",
				mcp.WithFinal(true), d.config.DiagnosticHint)
		}
		return d.config.FormatCommand(req, *command, params.CorrelationID)
	}
	command, found = d.commands.GetCommandResult(params.CorrelationID)
	if !found {
		return mcp.Fail(req, mcp.ErrNoData, "Command not found: "+params.CorrelationID,
			"The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid. Use observe with what='pending_commands' to see active commands.",
			mcp.WithFinal(true), d.config.DiagnosticHint)
	}
	return d.config.FormatCommand(req, *command, params.CorrelationID)
}

func (d *Dispatcher) pendingCommands(_ observecore.Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	if d.commands == nil {
		data := map[string]any{"pending": []*queries.CommandResult{}, "completed": []*queries.CommandResult{}, "failed": []*queries.CommandResult{}, "extension_in_progress": []syncruntime.SyncInProgress{}, "extension_in_progress_count": 0}
		return mcp.Succeed(req, "Pending: 0, Completed: 0, Failed: 0, Extension in-progress: 0", data)
	}
	pending, completed := d.commands.GetPendingCommands(), d.commands.GetCompletedCommands()
	failed := d.commands.GetFailedCommands()
	if pending == nil {
		pending = []*queries.CommandResult{}
	}
	if completed == nil {
		completed = []*queries.CommandResult{}
	}
	if failed == nil {
		failed = []*queries.CommandResult{}
	}
	inProgress := []syncruntime.SyncInProgress{}
	if d.config.InProgress != nil {
		inProgress = d.config.InProgress()
	}
	data := map[string]any{"pending": pending, "completed": completed, "failed": failed, "extension_in_progress": inProgress, "extension_in_progress_count": len(inProgress)}
	return mcp.Succeed(req, fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d, Extension in-progress: %d", len(pending), len(completed), len(failed), len(inProgress)), data)
}

func (d *Dispatcher) FailedCommands(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	if d.commands == nil {
		return mcp.Succeed(req, "No failed commands found", map[string]any{"commands": []*queries.CommandResult{}, "count": 0})
	}
	failed := d.commands.GetFailedCommands()
	data := map[string]any{"commands": failed, "count": len(failed)}
	if len(failed) == 0 {
		return mcp.Succeed(req, "No failed commands found", data)
	}
	return mcp.Succeed(req, fmt.Sprintf("Found %d failed/expired commands", len(failed)), data)
}

func (d *Dispatcher) savedVideos(_ observecore.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return screenrec.HandleObserveSavedVideos(req, args, d.config.StateDiagnostics)
}
