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
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]toolrouting.Handler[*ToolHandler]{
	"dom":                 azInspect(inspect.HandleDOM),
	"api_validation":      (*ToolHandler).toolValidateAPI,
	"page_summary":        (*ToolHandler).toolAnalyzePageSummary,
	"performance":         azObserve(observe.CheckPerformance),
	"accessibility":       azObserve(observe.RunA11yAudit),
	"error_clusters":      azObserve(observe.AnalyzeErrors),
	"navigation_patterns": azObserve(observe.AnalyzeHistory),
	"security_audit":      azLocal(toolanalyze.HandleSecurityAudit),
	"third_party_audit":   azLocal(toolanalyze.HandleThirdPartyAudit),
	"link_health":         azLocal(toolanalyze.HandleLinkHealth),
	"link_validation": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return toolanalyze.HandleLinkValidation(req, args, version)
	},
	"annotations": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.annotationAnalysis.GetAnnotations(req, args)
	},
	"annotation_detail": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.annotationAnalysis.GetAnnotationDetail(req, args)
	},
	"draw_history": func(_ *ToolHandler, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		dir, err := mediaapi.ScreenshotsDir()
		return annotation.ListDrawHistory(req, dir, err)
	},
	"draw_session": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		dir, err := mediaapi.ScreenshotsDir()
		return annotation.LoadDrawSession(h.annotationStore, req, args, dir, err)
	},
	"computed_styles":  azInspect(inspect.HandleComputedStyles),
	"forms":            azInspect(inspect.HandleFormDiscovery),
	"form_state":       azInspect(inspect.HandleFormState),
	"form_validation":  azInspect(inspect.HandleFormValidation),
	"data_table":       azInspect(inspect.HandleDataTable),
	"visual_baseline":  azVisual(visual.SaveBaseline),
	"visual_diff":      azVisual(visual.DiffBaseline),
	"visual_baselines": azVisual(visual.ListBaselines),
	"navigation":       azLocal(toolanalyze.HandleNavigation),
	"page_structure":   azLocal(toolanalyze.HandlePageStructure),
	"audit": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return combinedaudit.Handle(h, req, args)
	},
	"page_issues": azLocal(pageissues.Handle),
	"feature_gates": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.interactAction().HandleContentExtraction(req, args, "feature_gates", "feature_gates")
	},
}

func azObserve(fn func(observe.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[*ToolHandler] {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

// analyzeValueAliases maps shorthand names to their canonical analyze mode names with deprecation metadata.
var analyzeValueAliases = map[string]toolrouting.ValueAlias{
	"a11y":    {Canonical: "accessibility", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	"history": {Canonical: "navigation_patterns", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// analyzeAliasParams references the shared default mode/action aliases.
var analyzeAliasParams = toolrouting.DefaultModeActionAliases

// azLocal wraps a toolanalyze.Deps-accepting function as a toolrouting.Handler[*ToolHandler].
func azLocal(fn func(toolanalyze.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[*ToolHandler] {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func azInspect(fn func(inspect.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[*ToolHandler] {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func azVisual(fn func(visual.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) toolrouting.Handler[*ToolHandler] {
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
var analyzeRegistry = toolrouting.Registry[*ToolHandler]{
	Handlers:  analyzeHandlers,
	AliasDefs: analyzeAliasParams,
	Resolution: toolrouting.Resolution{
		ToolName:     "analyze",
		ValidModes:   "", // populated lazily
		ValueAliases: analyzeValueAliases,
	},
}

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	reg := analyzeRegistry
	reg.Resolution.ValidModes = getValidAnalyzeModes()
	return toolrouting.Dispatch(h, req, args, reg)
}

func (h *ToolHandler) toolAnalyzePageSummary(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.interactAction().HandleContentExtraction(req, args, "page_summary", "page_summary")
}

func (h *ToolHandler) toolValidateAPI(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.apiContractRuntime.Handle(req, args, h.capture.GetNetworkBodies())
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
