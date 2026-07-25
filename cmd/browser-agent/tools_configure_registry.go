// Purpose: Central registry for configure actions and valid-action metadata.
// Why: Keeps dispatch table updates isolated from argument parsing and wrapper logic.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

const defaultStoreNamespace = "session"

// configureHandlers maps configure action names to their handler functions.
var configureHandlers = map[string]ModeHandler{
	// Sub-handler delegates (require closures — configureSession() accessor)
	"store": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().handleConfigureStore(req, args)
	},
	"load": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().handleLoadSessionContext(req, args)
	},
	"diff_sessions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.configureSession().handleDiffSessionsWrapper(req, args)
	},
	// Args-less handlers (require closures — different receiver signature)
	"health": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolGetHealth(req)
	},
	"restart": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolConfigureRestart(req)
	},
	"doctor": func(h *ToolHandler, req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
		return h.toolDoctor(req)
	},
	// Direct method delegates
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
		return toolconfigure.HandleTutorial(h, req, args, tutorialFailureRecoveryPlaybooks())
	},
	"examples": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolconfigure.HandleTutorial(h, req, args, tutorialFailureRecoveryPlaybooks())
	},
	"save_sequence":       method((*ToolHandler).toolConfigureSaveSequence),
	"get_sequence":        method((*ToolHandler).toolConfigureGetSequence),
	"list_sequences":      method((*ToolHandler).toolConfigureListSequences),
	"delete_sequence":     method((*ToolHandler).toolConfigureDeleteSequence),
	"replay_sequence":     method((*ToolHandler).toolConfigureReplaySequence),
	"security_mode":       cfgLocal(toolconfigure.HandleSecurityMode),
	"network_recording":   method((*ToolHandler).toolConfigureNetworkRecording),
	"action_jitter":       cfgLocal(toolconfigure.HandleActionJitter),
	"report_issue":        method((*ToolHandler).toolConfigureReportIssue),
	"setup_quality_gates": method((*ToolHandler).toolConfigureSetupQualityGates),
}

// cfgLocal wraps a toolconfigure.Deps-accepting function as a ModeHandler.
func cfgLocal(fn func(toolconfigure.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

// getValidConfigureActions returns a sorted, comma-separated list of valid configure actions.
func getValidConfigureActions() string { return sortedMapKeys(configureHandlers) }
