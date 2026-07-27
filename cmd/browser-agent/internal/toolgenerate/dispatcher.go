// Purpose: Dispatches generate tool modes (reproduction, test, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify) and assembles output artifacts.
// Why: Acts as the top-level router for all artifact generation, delegating format-specific logic to sub-handlers.
// Docs: docs/features/feature/test-generation/index.md
package toolgenerate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate/annotations"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reproduction"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// TestGenerator owns context-based generation, healing, and classification.
type TestGenerator interface {
	HandleGenerateTestFromContext(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	HandleGenerateTestHeal(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	HandleGenerateTestClassify(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
}

// Dispatcher owns all generate-mode routing and artifacts.
type Dispatcher struct {
	deps  Deps
	tests TestGenerator
}

// NewDispatcher constructs the complete generate boundary.
func NewDispatcher(deps Deps, tests TestGenerator) *Dispatcher {
	return &Dispatcher{deps: deps, tests: tests}
}

// generateHandlers maps generate format names to their handler functions.
var generateHandlers = map[string]toolrouting.Handler[*Dispatcher]{
	// Direct method delegates
	"reproduction":      (*Dispatcher).getReproductionScript,
	"test":              (*Dispatcher).generateTest,
	"pr_summary":        (*Dispatcher).generatePRSummary,
	"sarif":             (*Dispatcher).ExportSARIF,
	"har":               (*Dispatcher).exportHAR,
	"csp":               (*Dispatcher).generateCSP,
	"sri":               (*Dispatcher).generateSRI,
	"visual_test":       (*Dispatcher).generateVisualTest,
	"annotation_report": (*Dispatcher).generateAnnotationReport,
	"annotation_issues": (*Dispatcher).generateAnnotationIssues,
	// Sub-handler delegates (require closures — testGen() accessor)
	"test_from_context": func(h *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.tests.HandleGenerateTestFromContext(req, args)
	},
	"test_heal": func(h *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.tests.HandleGenerateTestHeal(req, args)
	},
	"test_classify": func(h *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.tests.HandleGenerateTestClassify(req, args)
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
var generateAliasParams = []toolrouting.Alias{
	{JSONField: "format", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	{JSONField: "action", ConflictFn: isGenerateMode, FallbackFn: isGenerateMode, DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// getValidGenerateFormats returns a sorted, comma-separated list of valid generate formats.
func ValidFormats() string { return strings.Join(util.SortedMapKeys(generateHandlers), ", ") }

// generateRegistry is the tool registry for generate dispatch.
var generateRegistry = toolrouting.Registry[*Dispatcher]{
	Handlers:  generateHandlers,
	AliasDefs: generateAliasParams,
	Resolution: toolrouting.Resolution{
		ToolName:   "generate",
		ValidModes: "", // populated lazily
	},
	PreDispatch: func(_ *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *mcp.JSONRPCResponse) {
		return args, ValidateGenerateParams(req, what, args)
	},
}

// Handle dispatches generate requests based on the 'what' parameter.
func (h *Dispatcher) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	reg := generateRegistry
	reg.Resolution.ValidModes = ValidFormats()
	return toolrouting.Dispatch(h, req, args, reg)
}

func (h *Dispatcher) exportHAR(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return HandleExportHAR(h.deps, req, args)
}

func (h *Dispatcher) generatePRSummary(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return HandlePRSummary(h.deps, req, args)
}

func (h *Dispatcher) ExportSARIF(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return HandleExportSARIF(h.deps, req, args)
}

func (h *Dispatcher) generateCSP(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return HandleGenerateCSP(h.deps, req, args)
}

func (h *Dispatcher) generateSRI(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return HandleGenerateSRI(h.deps, req, args)
}

func (h *Dispatcher) generateTest(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return HandleGenerateTest(h.deps, req, args)
}

func (h *Dispatcher) generateVisualTest(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return annotations.HandleVisualTest(h.deps, req, args)
}

func (h *Dispatcher) generateAnnotationReport(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return annotations.HandleAnnotationReport(h.deps, req, args)
}

func (h *Dispatcher) generateAnnotationIssues(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return annotations.HandleAnnotationIssues(h.deps, req, args)
}

func (h *Dispatcher) getReproductionScript(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := reproduction.ParseParams(args)
	if err := reproduction.ValidateOutputFormat(params.OutputFormat); err != "" {
		return mcp.Fail(req, mcp.ErrInvalidParam, err, "Use 'kaboom' or 'playwright'", mcp.WithParam("output_format"))
	}
	allActions := h.deps.GetCapture().GetAllEnhancedActions()
	actions := reproduction.FilterLastN(allActions, params.LastN)
	script := reproduction.GenerateScript(actions, params)
	result := reproduction.BuildResult(script, params, actions, allActions)
	return mcp.Succeed(req, fmt.Sprintf("Reproduction script (%s, %d actions)", params.OutputFormat, len(actions)), result)
}
