// tools_configure.go — Defines the configure MCP mode boundary and its narrow dependencies.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/sequencehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/auditlog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qualitygates"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/tutorial"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const restartSelfSignalDelay = 100 * time.Millisecond

var replayMu sync.Mutex

var configureHandlers = map[string]ModeHandler{
	"store": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.configureSessions.Store(req, args)
	},
	"load": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.configureSessions.Load(req, args)
	},
	"diff_sessions": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.configureSessions.Diff(req, args)
	},
	"health": func(h *ToolHandler, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return h.toolGetHealth(req)
	},
	"restart": func(h *ToolHandler, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return h.toolConfigureRestart(req)
	},
	"doctor": func(h *ToolHandler, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return h.toolDoctor(req)
	},
	"noise_rule": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		rewrittenArgs, err := cfg.RewriteNoiseRuleArgs(args)
		if err != nil {
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
		}
		return toolconfigure.HandleNoise(h, req, rewrittenArgs)
	},
	"clear": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return toolconfigure.HandleClear(toolconfigure.ClearTargets{
			Capture: h.capture,
			ClearLogs: func() int {
				count := h.server.logs.EntryCount()
				h.server.logs.ClearEntries()
				return count
			},
			Inbox:       h.server.pushInbox,
			Annotations: h.annotationStore,
		}, req, args)
	},
	"audit_log":             method((*ToolHandler).toolGetAuditLog),
	"streaming":             method((*ToolHandler).toolConfigureStreaming),
	"test_boundary_start":   method((*ToolHandler).toolConfigureTestBoundaryStart),
	"test_boundary_end":     method((*ToolHandler).toolConfigureTestBoundaryEnd),
	"event_recording_start": method((*ToolHandler).toolConfigureEventRecordingStart),
	"event_recording_stop":  method((*ToolHandler).toolConfigureEventRecordingStop),
	"playback":              method((*ToolHandler).toolConfigurePlayback),
	"log_diff":              method((*ToolHandler).toolConfigureLogDiff),
	"telemetry":             cfgLocal(toolconfigure.HandleTelemetry),
	"describe_capabilities": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return toolconfigure.HandleDescribeCapabilities(h, req, args, version)
	},
	"tutorial": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return tutorial.HandleTutorial(h, req, args, playbooks.TutorialFailureRecoveryPlaybooks())
	},
	"examples": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return tutorial.HandleTutorial(h, req, args, playbooks.TutorialFailureRecoveryPlaybooks())
	},
	"save_sequence":     method((*ToolHandler).toolConfigureSaveSequence),
	"get_sequence":      method((*ToolHandler).toolConfigureGetSequence),
	"list_sequences":    method((*ToolHandler).toolConfigureListSequences),
	"delete_sequence":   method((*ToolHandler).toolConfigureDeleteSequence),
	"replay_sequence":   method((*ToolHandler).toolConfigureReplaySequence),
	"security_mode":     cfgLocal(toolconfigure.HandleSecurityMode),
	"network_recording": method((*ToolHandler).toolConfigureNetworkRecording),
	"action_jitter":     cfgLocal(toolconfigure.HandleActionJitter),
	"report_issue": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return issuereport.Handle(h, req, args)
	},
	"setup_quality_gates": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return qualitygates.Handle(h.server, req, args)
	},
}

func cfgLocal(fn func(toolconfigure.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func getValidConfigureActions() string { return sortedMapKeys(configureHandlers) }

func (h *ToolHandler) toolGetHealth(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	if h.healthMetrics == nil {
		return mcp.Fail(req, mcp.ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")
	}
	response := getHealthResponse(h.healthMetrics, h.capture, h.server, version)
	return mcp.Succeed(req, "Server health", response)
}

func (h *ToolHandler) toolDoctor(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	checks := health.RunDoctorChecks(h.capture)
	if h.healthMetrics != nil {
		uptime := h.healthMetrics.GetUptime()
		checks = append(checks, health.DoctorCheck{
			Name: "server_uptime", Status: "pass",
			Detail: fmt.Sprintf("Server running for %s (version %s)", uptime.Round(time.Second), version),
		})
	}
	overallStatus := "healthy"
	readyForInteraction := true
	for _, check := range checks {
		if check.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		} else if check.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}
	return mcp.Succeed(req, "Doctor: "+overallStatus, map[string]any{
		"status": overallStatus, "ready_for_interaction": readyForInteraction,
		"checks": checks, "hint": h.DiagnosticHintString(),
	})
}

type serverDepsAdapter struct{ s *Server }

func (a *serverDepsAdapter) GetTerminalPort() int {
	if a.s == nil {
		return 0
	}
	return a.s.getTerminalPort()
}

func (a *serverDepsAdapter) GetConsoleStats() (int, int, int64) {
	if a.s == nil || a.s.logs == nil {
		return 0, defaultMaxEntries, 0
	}
	return a.s.logs.EntryCount(), a.s.logs.MaxEntries(), a.s.logs.DropCount()
}

const defaultMaxEntries = 1000

func getHealthResponse(hm *health.Metrics, cap *capture.Store, server *Server, ver string) health.MCPHealthResponse {
	var serverDeps health.ServerDeps
	if server != nil {
		serverDeps = &serverDepsAdapter{s: server}
	}
	var upgrade health.UpgradeProvider
	if binaryUpgradeState != nil {
		upgrade = binaryUpgradeState
	}
	return hm.GetHealth(cap, serverDeps, upgrade, getLaunchModeInfo, ver)
}

func getLaunchModeInfo() health.LaunchModeInfo {
	lm := launchmode.Current()
	return health.LaunchModeInfo{
		Mode: lm.Mode, Reason: lm.Reason, ParentProcess: lm.ParentProcess,
	}
}

// configureAliasParams defines the deprecated alias parameters for the configure tool.
// "mode" is included for parity with observe and analyze. Both "mode" and "action" have
// ConflictFn and FallbackFn gates because these fields also serve as sub-parameters
// (e.g. security_mode uses "mode" as a field, playback uses "action" as a sub-action).
// Conflicts and fallbacks are only triggered when the value is a known top-level configure mode.
var configureAliasParams = []modeAlias{
	{JSONField: "mode", ConflictFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}, FallbackFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}, DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	{JSONField: "action", ConflictFn: func(v string) bool {
		_, ok := configureHandlers[v]
		return ok
	}, DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// configureRegistry is the tool registry for configure dispatch.
var configureRegistry = toolRegistry{
	Handlers:  configureHandlers,
	AliasDefs: configureAliasParams,
	Resolution: modeResolution{
		ToolName:   "configure",
		ValidModes: "", // populated lazily
	},
}

// toolConfigure dispatches configure requests based on the 'what' parameter.
func (h *ToolHandler) toolConfigure(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	reg := configureRegistry
	reg.Resolution.ValidModes = getValidConfigureActions()
	return h.dispatchTool(req, args, reg)
}

func (h *ToolHandler) NoiseConfig() *noise.NoiseConfig {
	return h.noiseConfig
}

func (h *ToolHandler) ConsoleEntries() []noise.LogEntry {
	snapshot := h.server.logs.Entries()
	entries := make([]noise.LogEntry, len(snapshot))
	for i, entry := range snapshot {
		entries[i] = noise.LogEntry(entry)
	}
	return entries
}

func (h *ToolHandler) NetworkBodies() []types.NetworkBody {
	return h.capture.GetNetworkBodies()
}

func (h *ToolHandler) AllWebSocketEvents() []types.WebSocketEvent {
	return h.capture.GetAllWebSocketEvents()
}

func (h *ToolHandler) GetTrackingStatus() (bool, int, string) {
	return h.capture.GetTrackingStatus()
}

func (h *ToolHandler) GetPilotStatus() any {
	return h.capture.GetPilotStatus()
}

func (h *ToolHandler) GetToolModuleExamples(toolName string) any {
	h.ensureToolModules()
	if module, ok := h.toolModules.get(toolName); ok {
		if examples := module.Examples(); len(examples) > 0 {
			return examples
		}
	}
	return nil
}

func (h *ToolHandler) GetSecurityMode() (string, bool, []string) {
	return h.capture.GetSecurityMode()
}

func (h *ToolHandler) SetSecurityMode(mode string, rewrites []string) {
	h.capture.SetSecurityMode(mode, rewrites)
}

func (h *ToolHandler) GetTelemetryMode() string {
	return h.server.logs.TelemetryMode()
}

func (h *ToolHandler) SetTelemetryMode(mode string) {
	h.server.logs.SetTelemetryMode(mode)
}

func (h *ToolHandler) InteractActionSetJitter(ms int) {
	h.interactAction().SetJitter(ms)
}

func (h *ToolHandler) InteractActionGetJitter() int {
	return h.interactAction().GetJitter()
}

func (h *ToolHandler) HasCapture() bool {
	return h.capture != nil
}

func (h *ToolHandler) CollectIssueReport(template, title, userContext string) issuereport.IssueReport {
	report := issuereport.IssueReport{Template: template, Title: title, UserContext: userContext}
	report.Diagnostics.Server.Version = version
	report.Diagnostics.Platform.OS = runtime.GOOS
	report.Diagnostics.Platform.Arch = runtime.GOARCH
	report.Diagnostics.Platform.GoVersion = runtime.Version()
	if h.healthMetrics != nil {
		report.Diagnostics.Server.UptimeSeconds = h.healthMetrics.GetUptime().Seconds()
		audit := h.healthMetrics.BuildAuditInfo()
		report.Diagnostics.Server.TotalCalls = audit.TotalCalls
		report.Diagnostics.Server.TotalErrors = audit.TotalErrors
		report.Diagnostics.Server.ErrorRatePct = audit.ErrorRatePct
	}
	if h.capture != nil {
		health := h.capture.GetHealthSnapshot()
		report.Diagnostics.Extension.Connected = health.ConnectionCount > 0
		report.Diagnostics.Extension.Source = health.ExtSessionID
		report.Diagnostics.Buffers.NetworkEntries = health.NetworkBodyCount
		report.Diagnostics.Buffers.ActionEntries = health.ActionCount
	}
	if h.server != nil {
		report.Diagnostics.Buffers.ConsoleEntries = h.server.logs.EntryCount()
	}
	return report
}

func (h *ToolHandler) SanitizeIssueReport(report issuereport.IssueReport) issuereport.IssueReport {
	if h.redactionEngine == nil {
		return report
	}
	return issuereport.NewSanitizer(h.redactionEngine).SanitizeReport(report)
}

func (h *ToolHandler) SubmitIssueReport(report issuereport.IssueReport) issuereport.SubmitResult {
	return issuereport.SubmitViaGH(h.shutdownCtx, report, h.issueCommandRunner)
}

func (h *ToolHandler) toolConfigureNetworkRecording(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return netrecord.HandleNetworkRecording(h.capture, h.networkRecording, req, args)
}

func extractErrorMessage(response mcp.JSONRPCResponse) string {
	if message := replay.ErrorMessage(response); message != "" {
		return message
	}
	return "unknown error"
}

func (h *ToolHandler) sequenceHandler() *sequencehandler.Handler {
	return sequencehandler.New(sequencehandler.Deps{
		Store: h.sessionStoreImpl, ReplayMu: &replayMu, Interact: h.toolInteract,
		WaitForCommand: h.capture.WaitForCommand, RecordAction: h.recordAIAction,
	})
}

func (h *ToolHandler) toolConfigureSaveSequence(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.sequenceHandler().Save(req, args)
}

func (h *ToolHandler) toolConfigureGetSequence(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.sequenceHandler().Get(req, args)
}

func (h *ToolHandler) toolConfigureListSequences(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.sequenceHandler().List(req, args)
}

func (h *ToolHandler) toolConfigureDeleteSequence(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.sequenceHandler().Delete(req, args)
}

func (h *ToolHandler) toolConfigureReplaySequence(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.sequenceHandler().Replay(req, args)
}

func (h *ToolHandler) toolConfigureEventRecordingStart(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.EventRecordingStart(req, args)
}

func (h *ToolHandler) toolConfigureEventRecordingStop(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.EventRecordingStop(req, args)
}

func (h *ToolHandler) toolConfigurePlayback(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.Playback(req, args)
}

func (h *ToolHandler) toolConfigureLogDiff(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.LogDiff(req, args)
}

func (h *ToolHandler) toolGetAuditLog(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, problem := auditlog.New(h.auditTrail).Execute(args)
	if problem != nil {
		switch problem.Kind {
		case auditlog.Unavailable:
			return mcp.Fail(req, mcp.ErrNotInitialized, problem.Message, "Internal error — do not retry")
		case auditlog.InvalidJSON:
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+problem.Message, "Fix JSON syntax and call again")
		case auditlog.InvalidOperation:
			return mcp.Fail(req, mcp.ErrInvalidParam, problem.Message, "Use operation: analyze, report, or clear", mcp.WithParam("operation"))
		default:
			return mcp.Fail(req, mcp.ErrInvalidParam, problem.Message, "Use RFC3339 format, for example 2026-02-17T15:04:05Z", mcp.WithParam("since"))
		}
	}

	switch result.Operation {
	case "clear":
		h.auditRecorder.ResetSessions()
		return mcp.Succeed(req, "Audit log cleared", map[string]any{
			"status": "ok", "operation": result.Operation, "cleared": result.Cleared,
		})
	case "analyze":
		return mcp.Succeed(req, "Audit log analysis", map[string]any{
			"status": "ok", "operation": result.Operation, "summary": result.Summary,
		})
	default:
		return mcp.Succeed(req, "Audit log entries", map[string]any{
			"status": "ok", "operation": result.Operation, "entries": result.Entries, "count": result.Count,
		})
	}
}

func (h *ToolHandler) toolConfigureStreaming(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	rewritten, err := cfg.RewriteStreamingArgs(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	var params struct {
		Action          string   `json:"action"`
		Events          []string `json:"events"`
		ThrottleSeconds int      `json:"throttle_seconds"`
		URLFilter       string   `json:"url"`
		SeverityMin     string   `json:"severity_min"`
	}
	if resp, stop := mcp.ParseArgs(req, rewritten, &params); stop {
		return resp
	}
	if resp, blocked := requireString(req, params.Action, "action", "Add the 'action' parameter and call again"); blocked {
		return resp
	}
	result := h.alertBuffer.Stream.Configure(
		params.Action,
		params.Events,
		params.ThrottleSeconds,
		params.URLFilter,
		params.SeverityMin,
	)
	return mcp.Succeed(req, "Streaming configuration", result)
}

func (h *ToolHandler) toolConfigureRestart(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	resp := mcp.Succeed(req, "Daemon restarting", map[string]any{
		"status":    "ok",
		"restarted": true,
		"message":   "Daemon shutting down — bridge will respawn automatically",
	})
	util.SafeGo(func() {
		time.Sleep(restartSelfSignalDelay)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGTERM)
	})
	return resp
}

func (h *ToolHandler) toolConfigureTestBoundaryStart(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryStart(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	h.activeBoundariesMu.Lock()
	defer h.activeBoundariesMu.Unlock()
	if h.activeBoundaries == nil {
		h.activeBoundaries = make(map[string]time.Time)
	}
	h.activeBoundaries[result.TestID] = time.Now()
	return cfg.BuildTestBoundaryStartResponse(req.ID, result)
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryEnd(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	h.activeBoundariesMu.Lock()
	_, wasActive := h.activeBoundaries[result.TestID]
	if wasActive {
		delete(h.activeBoundaries, result.TestID)
	}
	h.activeBoundariesMu.Unlock()
	if !wasActive {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"No active test boundary for test_id '"+result.TestID+"'",
			"Call configure({what: 'test_boundary_start', test_id: '"+result.TestID+"'}) first",
			mcp.WithParam("test_id"))
	}
	return cfg.BuildTestBoundaryEndResponse(req.ID, result, wasActive)
}
