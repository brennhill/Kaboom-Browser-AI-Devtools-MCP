// Purpose: Centralizes analyze mode routing, aliases, and canonical-mode validation.
// Why: Keeps top-level dispatch concerns separated from mode-specific analyze handlers.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/analyzehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]ModeHandler{
	"dom":                 method((*ToolHandler).toolQueryDOM),
	"api_validation":      method((*ToolHandler).toolValidateAPI),
	"page_summary":        method((*ToolHandler).toolAnalyzePageSummary),
	"performance":         obs(observe.CheckPerformance),
	"accessibility":       obs(observe.RunA11yAudit),
	"error_clusters":      obs(observe.AnalyzeErrors),
	"navigation_patterns": obs(observe.AnalyzeHistory),
	"security_audit":      azLocal(analyzehandler.HandleSecurityAudit),
	"third_party_audit":   azLocal(analyzehandler.HandleThirdPartyAudit),
	"link_health":         azLocal(analyzehandler.HandleLinkHealth),
	"link_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return analyzehandler.HandleLinkValidation(req, args, version)
	},
	"annotations":       method((*ToolHandler).toolGetAnnotations),
	"annotation_detail": method((*ToolHandler).toolGetAnnotationDetail),
	"draw_history":      method((*ToolHandler).toolListDrawHistory),
	"draw_session":      method((*ToolHandler).toolGetDrawSession),
	"computed_styles":   toolComputedStyles,
	"forms":             toolFormDiscovery,
	"form_state":        toolFormState,
	"form_validation":   toolFormValidation,
	"data_table":        toolDataTable,
	"visual_baseline":   method((*ToolHandler).toolVisualBaseline),
	"visual_diff":       method((*ToolHandler).toolVisualDiff),
	"visual_baselines":  method((*ToolHandler).toolListVisualBaselines),
	"navigation":        azLocal(analyzehandler.HandleNavigation),
	"page_structure":    azLocal(analyzehandler.HandlePageStructure),
	"audit":             method((*ToolHandler).toolAnalyzeAudit),
	"page_issues":       method((*ToolHandler).toolAnalyzePageIssues),
	"feature_gates": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

// azLocal wraps a analyzehandler.Deps-accepting function as a ModeHandler.
func azLocal(fn func(analyzehandler.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
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
func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := analyzeRegistry
	reg.Resolution.ValidModes = getValidAnalyzeModes()
	return h.dispatchTool(req, args, reg)
}
