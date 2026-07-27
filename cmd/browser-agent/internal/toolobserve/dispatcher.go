// dispatcher.go — Owns observe mode routing and server-side command observation.
// Docs: docs/features/feature/observe/index.md

package toolobserve

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	observe "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const annotationCommandWaitTimeout = 55 * time.Second

type Host interface {
	Deps
	observe.Deps
}

type CommandStore interface {
	WaitForCommand(string, time.Duration) (*queries.CommandResult, bool)
	GetCommandResult(string) (*queries.CommandResult, bool)
	GetPendingCommands() []*queries.CommandResult
	GetCompletedCommands() []*queries.CommandResult
	GetFailedCommands() []*queries.CommandResult
	GetInProgressCommands() []capture.SyncInProgress
}

type Config struct {
	Host             Host
	Commands         CommandStore
	AnnotationStore  *annotation.Store
	Annotations      toolrouting.Handler[Host]
	AnnotationDetail toolrouting.Handler[Host]
	Recordings       toolrouting.Handler[Host]
	RecordingActions toolrouting.Handler[Host]
	PlaybackResults  toolrouting.Handler[Host]
	LogDiffReport    toolrouting.Handler[Host]
	FormatCommand    func(mcp.JSONRPCRequest, queries.CommandResult, string) mcp.JSONRPCResponse
	InjectSummary    func(json.RawMessage) json.RawMessage
	DrainAlerts      func() []types.Alert
	DiagnosticHint   func(*mcp.StructuredError)
}

type Dispatcher struct {
	host     Host
	commands CommandStore
	config   Config
	registry toolrouting.Registry[Host]
}

func NewDispatcher(config Config) *Dispatcher {
	d := &Dispatcher{host: config.Host, commands: config.Commands, config: config}
	handlers := map[string]toolrouting.Handler[Host]{
		"errors": wrap(observe.GetBrowserErrors), "logs": wrap(observe.GetBrowserLogs),
		"extension_logs": wrap(observe.GetExtensionLogs), "network_waterfall": wrap(observe.GetNetworkWaterfall),
		"network_bodies": wrap(observe.GetNetworkBodies), "websocket_events": wrap(observe.GetWSEvents),
		"websocket_status": wrap(observe.GetWSStatus), "actions": wrap(observe.GetEnhancedActions),
		"vitals": wrap(observe.GetWebVitals), "page": wrap(observe.GetPageInfo), "tabs": wrap(observe.GetTabs),
		"history": wrap(observe.AnalyzeHistory), "pilot": wrap(observe.ObservePilot),
		"timeline": wrap(observe.GetSessionTimeline), "error_bundles": wrap(observe.GetErrorBundles),
		"screenshot": wrap(observe.GetScreenshot), "storage": wrap(observe.GetStorage),
		"indexeddb": wrap(observe.GetIndexedDB), "summarized_logs": wrap(observe.GetSummarizedLogs),
		"transients":  wrap(observe.GetTransients),
		"annotations": config.Annotations, "annotation_detail": config.AnnotationDetail,
		"draw_history": d.drawHistory, "draw_session": d.drawSession,
		"page_inventory": local(HandlePageInventory), "inbox": local(HandleInbox), "site_menus": local(HandleSiteMenus),
		"command_result": func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.CommandResult(req, args)
		},
		"pending_commands": d.pendingCommands,
		"failed_commands": func(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return d.FailedCommands(req, args)
		},
		"saved_videos": d.savedVideos,
		"recordings":   config.Recordings, "recording_actions": config.RecordingActions,
		"playback_results": config.PlaybackResults, "log_diff_report": config.LogDiffReport,
	}
	d.registry = toolrouting.Registry[Host]{
		Handlers: handlers, AliasDefs: toolrouting.DefaultModeActionAliases,
		Resolution: toolrouting.Resolution{
			ToolName: "observe", ValidModes: strings.Join(util.SortedMapKeys(handlers), ", "),
			ValueAliases: map[string]toolrouting.ValueAlias{
				"network": {Canonical: "network_waterfall", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
				"ws":      {Canonical: "websocket_events", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
			},
		},
		PreDispatch: func(_ Host, _ mcp.JSONRPCRequest, args json.RawMessage, _ string) (json.RawMessage, *mcp.JSONRPCResponse) {
			return config.InjectSummary(args), nil
		},
		PostDispatch: func(h Host, _ mcp.JSONRPCRequest, resp mcp.JSONRPCResponse, what string) mcp.JSONRPCResponse {
			if !h.IsExtensionConnected() && !ServerSideObserveModes[what] {
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

func wrap(fn func(observe.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[Host] {
	return func(h Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func local(fn func(Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[Host] {
	return func(h Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func (d *Dispatcher) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return toolrouting.Dispatch(d.host, req, args, d.registry)
}

func (d *Dispatcher) ValidModes() []string { return util.SortedMapKeys(d.registry.Handlers) }

func (d *Dispatcher) drawHistory(_ Host, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.ListDrawHistory(req, dir, err)
}

func (d *Dispatcher) drawSession(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.LoadDrawSession(d.config.AnnotationStore, req, args, dir, err)
}

func (d *Dispatcher) CommandResult(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
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

func (d *Dispatcher) pendingCommands(_ Host, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	pending, completed := d.commands.GetPendingCommands(), d.commands.GetCompletedCommands()
	failed, inProgress := d.commands.GetFailedCommands(), d.commands.GetInProgressCommands()
	data := map[string]any{"pending": pending, "completed": completed, "failed": failed, "extension_in_progress": inProgress, "extension_in_progress_count": len(inProgress)}
	return mcp.Succeed(req, fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d, Extension in-progress: %d", len(pending), len(completed), len(failed), len(inProgress)), data)
}

func (d *Dispatcher) FailedCommands(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	failed := d.commands.GetFailedCommands()
	data := map[string]any{"commands": failed, "count": len(failed)}
	if len(failed) == 0 {
		return mcp.Succeed(req, "No failed commands found", data)
	}
	return mcp.Succeed(req, fmt.Sprintf("Found %d failed/expired commands", len(failed)), data)
}

func (d *Dispatcher) savedVideos(_ Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return screenrec.HandleObserveSavedVideos(req, args)
}
