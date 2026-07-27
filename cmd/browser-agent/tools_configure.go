// tools_configure.go — Defines the configure MCP mode boundary and its narrow dependencies.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
	"runtime"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qualitygates"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/tutorial"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const defaultStoreNamespace = "session"

var configureHandlers = map[string]ModeHandler{
	"store": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().handleConfigureStore(req, args)
	},
	"load": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().handleLoadSessionContext(req, args)
	},
	"diff_sessions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().handleDiffSessionsWrapper(req, args)
	},
	"health": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolGetHealth(req)
	},
	"restart": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolConfigureRestart(req)
	},
	"doctor": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolDoctor(req)
	},
	"noise_rule": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		rewrittenArgs, err := cfg.RewriteNoiseRuleArgs(args)
		if err != nil {
			return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
		}
		return toolconfigure.HandleNoise(h, req, rewrittenArgs)
	},
	"clear":                 method((*ToolHandler).toolConfigureClear),
	"audit_log":             method((*ToolHandler).toolGetAuditLog),
	"streaming":             method((*ToolHandler).toolConfigureStreaming),
	"test_boundary_start":   method((*ToolHandler).toolConfigureTestBoundaryStart),
	"test_boundary_end":     method((*ToolHandler).toolConfigureTestBoundaryEnd),
	"event_recording_start": method((*ToolHandler).toolConfigureEventRecordingStart),
	"event_recording_stop":  method((*ToolHandler).toolConfigureEventRecordingStop),
	"playback":              method((*ToolHandler).toolConfigurePlayback),
	"log_diff":              method((*ToolHandler).toolConfigureLogDiff),
	"telemetry":             cfgLocal(toolconfigure.HandleTelemetry),
	"describe_capabilities": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolconfigure.HandleDescribeCapabilities(h, req, args, version)
	},
	"tutorial": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return tutorial.HandleTutorial(h, req, args, tutorialFailureRecoveryPlaybooks())
	},
	"examples": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return tutorial.HandleTutorial(h, req, args, tutorialFailureRecoveryPlaybooks())
	},
	"save_sequence":     method((*ToolHandler).toolConfigureSaveSequence),
	"get_sequence":      method((*ToolHandler).toolConfigureGetSequence),
	"list_sequences":    method((*ToolHandler).toolConfigureListSequences),
	"delete_sequence":   method((*ToolHandler).toolConfigureDeleteSequence),
	"replay_sequence":   method((*ToolHandler).toolConfigureReplaySequence),
	"security_mode":     cfgLocal(toolconfigure.HandleSecurityMode),
	"network_recording": method((*ToolHandler).toolConfigureNetworkRecording),
	"action_jitter":     cfgLocal(toolconfigure.HandleActionJitter),
	"report_issue": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return issuereport.Handle(h, req, args)
	},
	"setup_quality_gates": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return qualitygates.Handle(h.server, req, args)
	},
}

func cfgLocal(fn func(toolconfigure.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

func getValidConfigureActions() string { return sortedMapKeys(configureHandlers) }

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
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := configureRegistry
	reg.Resolution.ValidModes = getValidConfigureActions()
	return h.dispatchTool(req, args, reg)
}

func isStoreAction(action string) bool {
	switch action {
	case "save", "load", "list", "delete", "stats":
		return true
	default:
		return false
	}
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
