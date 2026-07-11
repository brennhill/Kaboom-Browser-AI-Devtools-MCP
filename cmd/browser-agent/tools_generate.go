// Purpose: Dispatches generate tool modes (reproduction, test, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify) and assembles output artifacts.
// Why: Acts as the top-level router for all artifact generation, delegating format-specific logic to sub-handlers.
// Docs: docs/features/feature/test-generation/index.md
package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
	gen "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/generate"
)

// generateHandlers maps generate format names to their handler functions.
var generateHandlers = map[string]ModeHandler{
	// Direct method delegates
	"reproduction":      method((*ToolHandler).toolGetReproductionScript),
	"test":              method((*ToolHandler).toolGenerateTest),
	"pr_summary":        method((*ToolHandler).toolGeneratePRSummary),
	"sarif":             method((*ToolHandler).toolExportSARIF),
	"har":               method((*ToolHandler).toolExportHAR),
	"csp":               method((*ToolHandler).toolGenerateCSP),
	"sri":               method((*ToolHandler).toolGenerateSRI),
	"visual_test":       method((*ToolHandler).toolGenerateVisualTest),
	"annotation_report": method((*ToolHandler).toolGenerateAnnotationReport),
	"annotation_issues": method((*ToolHandler).toolGenerateAnnotationIssues),
	// Sub-handler delegates (require closures — testGen() accessor)
	"test_from_context": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.testGen().handleGenerateTestFromContext(req, args)
	},
	"test_heal": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.testGen().handleGenerateTestHeal(req, args)
	},
	"test_classify": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.testGen().handleGenerateTestClassify(req, args)
	},
}

// isGenerateMode returns true when the value is a known top-level generate mode.
func isGenerateMode(v string) bool {
	_, ok := generateHandlers[v]
	return ok
}

// generateAliasParams defines the deprecated alias parameters for the generate tool.
// "action" is only treated as a mode alias when its value matches a known generate mode,
// since "action" can also be a sub-action parameter (e.g. test_heal action=analyze).
// Both ConflictFn and FallbackFn are gated to handler membership.
var generateAliasParams = []modeAlias{
	{JSONField: "format", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	{JSONField: "action", ConflictFn: isGenerateMode, FallbackFn: isGenerateMode, DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// getValidGenerateFormats returns a sorted, comma-separated list of valid generate formats.
func getValidGenerateFormats() string { return sortedMapKeys(generateHandlers) }

// generateRegistry is the tool registry for generate dispatch.
var generateRegistry = toolRegistry{
	Handlers:  generateHandlers,
	AliasDefs: generateAliasParams,
	Resolution: modeResolution{
		ToolName:   "generate",
		ValidModes: "", // populated lazily
	},
	PreDispatch: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *JSONRPCResponse) {
		return args, generatehandler.ValidateGenerateParams(req, what, args)
	},
}

// toolGenerate dispatches generate requests based on the 'what' parameter.
func (h *ToolHandler) toolGenerate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := generateRegistry
	reg.Resolution.ValidModes = getValidGenerateFormats()
	return h.dispatchTool(req, args, reg)
}

// TestGenParams delegates to internal/tools/generate.
type TestGenParams = gen.TestGenParams
