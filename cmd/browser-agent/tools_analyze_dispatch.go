// Purpose: Centralizes analyze mode routing, aliases, and canonical-mode validation.
// Why: Keeps top-level dispatch concerns separated from mode-specific analyze handlers.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/combinedaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/inspect"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/pageissues"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/visual"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]ModeHandler{
	"dom":                 azInspect(inspect.HandleDOM),
	"api_validation":      method((*ToolHandler).toolValidateAPI),
	"page_summary":        method((*ToolHandler).toolAnalyzePageSummary),
	"performance":         obs(observe.CheckPerformance),
	"accessibility":       obs(observe.RunA11yAudit),
	"error_clusters":      obs(observe.AnalyzeErrors),
	"navigation_patterns": obs(observe.AnalyzeHistory),
	"security_audit":      azLocal(toolanalyze.HandleSecurityAudit),
	"third_party_audit":   azLocal(toolanalyze.HandleThirdPartyAudit),
	"link_health":         azLocal(toolanalyze.HandleLinkHealth),
	"link_validation": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return toolanalyze.HandleLinkValidation(req, args, version)
	},
	"annotations":       method((*ToolHandler).toolGetAnnotations),
	"annotation_detail": method((*ToolHandler).toolGetAnnotationDetail),
	"draw_history":      method((*ToolHandler).toolListDrawHistory),
	"draw_session":      method((*ToolHandler).toolGetDrawSession),
	"computed_styles":   azInspect(inspect.HandleComputedStyles),
	"forms":             azInspect(inspect.HandleFormDiscovery),
	"form_state":        azInspect(inspect.HandleFormState),
	"form_validation":   azInspect(inspect.HandleFormValidation),
	"data_table":        azInspect(inspect.HandleDataTable),
	"visual_baseline":   azVisual(visual.SaveBaseline),
	"visual_diff":       azVisual(visual.DiffBaseline),
	"visual_baselines":  azVisual(visual.ListBaselines),
	"navigation":        azLocal(toolanalyze.HandleNavigation),
	"page_structure":    azLocal(toolanalyze.HandlePageStructure),
	"audit": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return combinedaudit.Handle(h, req, args)
	},
	"page_issues": azLocal(pageissues.Handle),
	"feature_gates": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.interactAction().HandleContentExtraction(req, args, "feature_gates", "feature_gates")
	},
}

// analyzeValueAliases maps shorthand names to their canonical analyze mode names with deprecation metadata.
var analyzeValueAliases = map[string]modeValueAlias{
	"a11y":    {Canonical: "accessibility", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	"history": {Canonical: "navigation_patterns", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// analyzeAliasParams references the shared default mode/action aliases.
var analyzeAliasParams = defaultModeActionAliases

// azLocal wraps a toolanalyze.Deps-accepting function as a ModeHandler.
func azLocal(fn func(toolanalyze.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func azInspect(fn func(inspect.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func azVisual(fn func(visual.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(visualAnalyzeDeps{h: h}, req, args)
	}
}

type visualAnalyzeDeps struct{ h *ToolHandler }

func (d visualAnalyzeDeps) CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	return observe.GetScreenshot(d.h, req, json.RawMessage(`{}`))
}

func (d visualAnalyzeDeps) GetTrackingStatus() (bool, int, string) {
	return d.h.capture.GetTrackingStatus()
}

func (d visualAnalyzeDeps) HasSessionStore() bool {
	return d.h.sessionStoreImpl != nil
}

func (d visualAnalyzeDeps) HandleSessionStore(args persistence.SessionStoreArgs) (json.RawMessage, error) {
	return d.h.sessionStoreImpl.HandleSessionStore(args)
}

// getValidAnalyzeModes returns a sorted, comma-separated list of valid analyze modes.
func getValidAnalyzeModes() string { return sortedMapKeys(analyzeHandlers) }

// analyzeRegistry is the tool registry for analyze dispatch.
var analyzeRegistry = toolRegistry{
	Handlers:  analyzeHandlers,
	AliasDefs: analyzeAliasParams,
	Resolution: modeResolution{
		ToolName:     "analyze",
		ValidModes:   "", // populated lazily
		ValueAliases: analyzeValueAliases,
	},
}

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	reg := analyzeRegistry
	reg.Resolution.ValidModes = getValidAnalyzeModes()
	return h.dispatchTool(req, args, reg)
}

func (h *ToolHandler) toolAnalyzePageSummary(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.interactAction().HandleContentExtraction(req, args, "page_summary", "page_summary")
}

func (h *ToolHandler) toolValidateAPI(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.apiContractRuntime.Handle(req, args, h.capture.GetNetworkBodies())
}

func (h *ToolHandler) toolListDrawHistory(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.ListDrawHistory(req, dir, err)
}

func (h *ToolHandler) toolGetDrawSession(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	dir, err := mediaapi.ScreenshotsDir()
	return annotation.LoadDrawSession(h.annotationStore, req, args, dir, err)
}

func (h *ToolHandler) NetworkWaterfallEntries() []capture.NetworkWaterfallEntry {
	return h.capture.GetNetworkWaterfallEntries()
}

func (h *ToolHandler) ConsoleSecurityEntries() []scan.LogEntry {
	snapshot := h.server.logs.Entries()
	entries := make([]scan.LogEntry, len(snapshot))
	for index, entry := range snapshot {
		entries[index] = scan.LogEntry(entry)
	}
	return entries
}

func (h *ToolHandler) SecurityScanner() toolanalyze.SecurityScannerInterface {
	if h.securityScannerImpl == nil {
		return nil
	}
	return h.securityScannerImpl
}

func (h *ToolHandler) LogEntries() []types.LogEntry {
	entries, _ := h.GetLogEntries()
	return entries
}
