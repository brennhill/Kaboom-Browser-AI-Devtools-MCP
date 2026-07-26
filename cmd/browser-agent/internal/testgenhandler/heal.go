// Purpose: Handles test_heal mode — analyzes broken selectors and repairs test files with updated locators.
// Why: Automates test maintenance by detecting stale selectors and applying high-confidence fixes.
// Docs: docs/features/feature/self-healing-tests/index.md

package testgenhandler

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func (h *Handler) HandleGenerateTestHeal(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params TestHealRequest

	warnings, err := mcp.UnmarshalWithWarnings(args, &params)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	warnings = toolgenerate.FilterGenerateDispatchWarnings(warnings)

	if errResp, ok := validateHealParams(req, params); ok {
		return errResp
	}

	projectDir, _ := os.Getwd()

	result, errResp, ok := h.dispatchHealAction(req, params, projectDir)
	if ok {
		return errResp
	}

	summary := formatHealSummary(params, result)
	data := map[string]any{"result": result}
	resp := mcp.Succeed(req, summary, data)
	return mcp.AppendWarningsToResponse(resp, warnings)
}

var validHealActions = []string{"analyze", "repair", "batch"}

func validateHealParams(req mcp.JSONRPCRequest, params TestHealRequest) (mcp.JSONRPCResponse, bool) {
	if resp, blocked := toolresp.RequireString(req, params.Action, "action", "Add the 'action' parameter and call again"); blocked {
		return resp, true
	}
	if resp, blocked := toolresp.RequireOneOf(req, params.Action, "action", validHealActions, "Use a valid action value"); blocked {
		return resp, true
	}
	return mcp.JSONRPCResponse{}, false
}

func (h *Handler) dispatchHealAction(req mcp.JSONRPCRequest, params TestHealRequest, projectDir string) (any, mcp.JSONRPCResponse, bool) {
	switch params.Action {
	case "analyze":
		return h.handleHealAnalyze(req, params, projectDir)
	case "repair":
		return h.handleHealRepair(req, params, projectDir)
	case "batch":
		return h.handleHealBatch(req, params, projectDir)
	}
	return nil, mcp.JSONRPCResponse{}, false
}

func (h *Handler) handleHealAnalyze(req mcp.JSONRPCRequest, params TestHealRequest, projectDir string) (any, mcp.JSONRPCResponse, bool) {
	if resp, blocked := toolresp.RequireString(req, params.TestFile, "test_file", "Add the 'test_file' parameter and call again"); blocked {
		return nil, resp, true
	}
	selectors, err := h.analyzeTestFile(params, projectDir)
	if err != nil {
		return nil, mapAnalyzeError(req, params, err), true
	}
	result := map[string]any{
		"broken_selectors": selectors,
		"count":            len(selectors),
	}
	return result, mcp.JSONRPCResponse{}, false
}

func mapAnalyzeError(req mcp.JSONRPCRequest, params TestHealRequest, err error) mcp.JSONRPCResponse {
	errMsg := err.Error()
	if strings.Contains(errMsg, ErrTestFileNotFound) {
		return mcp.Fail(req, ErrTestFileNotFound, "Test file not found: "+params.TestFile, "Check the file path and try again")
	}
	if strings.Contains(errMsg, mcp.ErrPathNotAllowed) {
		return mcp.Fail(req, mcp.ErrPathNotAllowed, errMsg, "Use a path within the project directory")
	}
	return mcp.Fail(req, mcp.ErrInternal, "Failed to analyze test file: "+errMsg, "Check that the test file path is valid and readable")
}

func (h *Handler) handleHealRepair(req mcp.JSONRPCRequest, params TestHealRequest, projectDir string) (any, mcp.JSONRPCResponse, bool) {
	if len(params.BrokenSelectors) == 0 {
		return nil, mcp.Fail(req, mcp.ErrMissingParam,
			"Required parameter 'broken_selectors' is missing for repair action",
			"Add the 'broken_selectors' parameter and call again",
			mcp.WithParam("broken_selectors"),
		), true
	}
	healResult, err := h.repairSelectors(params, projectDir)
	if err != nil {
		return nil, mcp.Fail(req, mcp.ErrInternal, "Failed to repair selectors: "+err.Error(), "Check the broken_selectors input and retry"), true
	}
	return healResult, mcp.JSONRPCResponse{}, false
}

func (h *Handler) handleHealBatch(req mcp.JSONRPCRequest, params TestHealRequest, projectDir string) (any, mcp.JSONRPCResponse, bool) {
	if resp, blocked := toolresp.RequireString(req, params.TestDir, "test_dir", "Add the 'test_dir' parameter and call again"); blocked {
		return nil, resp, true
	}
	batchResult, err := h.healTestBatch(params, projectDir)
	if err != nil {
		return nil, mapBatchError(req, err), true
	}
	return batchResult, mcp.JSONRPCResponse{}, false
}

func mapBatchError(req mcp.JSONRPCRequest, err error) mcp.JSONRPCResponse {
	errMsg := err.Error()
	if strings.Contains(errMsg, mcp.ErrPathNotAllowed) {
		return mcp.Fail(req, mcp.ErrPathNotAllowed, errMsg, "Use a path within the project directory")
	}
	if strings.Contains(errMsg, ErrBatchTooLarge) {
		return mcp.Fail(req, ErrBatchTooLarge, errMsg, "Reduce the number or size of test files")
	}
	return mcp.Fail(req, mcp.ErrInternal, "Failed to heal test batch: "+errMsg, "Check the test_dir path and file permissions, then retry")
}
