// Purpose: Handles test_classify mode — classifies test failures as flaky, environment, assertion, or crash using failure signatures.
// Why: Enables agents to triage failing tests automatically without manual inspection.
// Docs: docs/features/feature/self-healing-tests/index.md

package testgenhandler

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func (h *Handler) HandleGenerateTestClassify(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params TestClassifyRequest

	warnings, err := mcp.UnmarshalWithWarnings(args, &params)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	warnings = toolgenerate.FilterGenerateDispatchWarnings(warnings)

	if errResp, blocked := validateClassifyParams(req, params); blocked {
		return errResp
	}

	result, summary, errResp := h.dispatchClassifyAction(req.ID, params)
	if errResp != nil {
		return *errResp
	}

	resp := mcp.Succeed(req, summary, result)
	return mcp.AppendWarningsToResponse(resp, warnings)
}

func (h *Handler) dispatchClassifyAction(reqID any, params TestClassifyRequest) (any, string, *mcp.JSONRPCResponse) {
	switch params.Action {
	case "failure":
		result, summary, errResp, ok := h.classifySingleFailure(reqID, params)
		if !ok {
			return nil, "", &errResp
		}
		return result, summary, nil
	case "batch":
		result, summary, errResp, ok := h.classifyBatchFailures(reqID, params)
		if !ok {
			return nil, "", &errResp
		}
		return result, summary, nil
	}
	return nil, "", nil
}

var validClassifyActions = []string{"failure", "batch"}

func validateClassifyParams(req mcp.JSONRPCRequest, params TestClassifyRequest) (mcp.JSONRPCResponse, bool) {
	if resp, blocked := toolresp.RequireString(req, params.Action, "action", "Add the 'action' parameter and call again"); blocked {
		return resp, true
	}
	if resp, blocked := toolresp.RequireOneOf(req, params.Action, "action", validClassifyActions, "Use a valid action value"); blocked {
		return resp, true
	}
	return mcp.JSONRPCResponse{}, false
}

func (h *Handler) classifySingleFailure(reqID any, params TestClassifyRequest) (any, string, mcp.JSONRPCResponse, bool) {
	if params.Failure == nil {
		return nil, "", mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      reqID,
			Result: mcp.StructuredErrorResponse(
				mcp.ErrMissingParam,
				"Required parameter 'failure' is missing for failure action",
				"Add the 'failure' parameter and call again",
				mcp.WithParam("failure"),
			),
		}, false
	}

	classification := h.classifyFailure(params.Failure)

	if classification.Confidence < 0.5 {
		return nil, "", mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      reqID,
			Result: mcp.StructuredErrorResponse(
				ErrClassificationUncertain,
				fmt.Sprintf("Could not classify failure with sufficient confidence (%.2f < 0.50)", classification.Confidence),
				"Provide more context or manually review the failure",
				mcp.WithHint("Category: "+classification.Category),
			),
		}, false
	}

	summary := fmt.Sprintf("Classified as %s (%.0f%% confidence) — recommended: %s",
		classification.Category,
		classification.Confidence*100,
		classification.RecommendedAction)

	data := map[string]any{"classification": classification}
	if classification.SuggestedFix != nil {
		data["suggested_fix"] = classification.SuggestedFix
	}
	return data, summary, mcp.JSONRPCResponse{}, true
}

func (h *Handler) classifyBatchFailures(reqID any, params TestClassifyRequest) (any, string, mcp.JSONRPCResponse, bool) {
	if len(params.Failures) == 0 {
		return nil, "", mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      reqID,
			Result: mcp.StructuredErrorResponse(
				mcp.ErrMissingParam,
				"Required parameter 'failures' is missing for batch action",
				"Add the 'failures' parameter and call again",
				mcp.WithParam("failures"),
			),
		}, false
	}

	if len(params.Failures) > maxFailuresPerBatch {
		return nil, "", mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      reqID,
			Result: mcp.StructuredErrorResponse(
				ErrBatchTooLarge,
				fmt.Sprintf("Batch contains %d failures, max is %d", len(params.Failures), maxFailuresPerBatch),
				"Reduce the number of failures and try again",
			),
		}, false
	}

	batchResult := h.classifyFailureBatch(params.Failures)

	summary := fmt.Sprintf("Classified %d failures: %d real bugs, %d flaky, %d test issues, %d uncertain",
		batchResult.TotalClassified,
		batchResult.RealBugs,
		batchResult.FlakyTests,
		batchResult.TestBugs,
		batchResult.Uncertain)

	result := map[string]any{"batch_result": batchResult}
	return result, summary, mcp.JSONRPCResponse{}, true
}
