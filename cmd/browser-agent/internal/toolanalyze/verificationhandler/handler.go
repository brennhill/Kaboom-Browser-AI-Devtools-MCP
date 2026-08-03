// handler.go — Defines and evaluates versioned QA verification contracts.
// Docs: docs/features/feature/verification-contracts/index.md

package verificationhandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
)

type params struct {
	Operation string                         `json:"operation"`
	Contract  verification.Contract          `json:"contract"`
	Results   []verification.AssertionResult `json:"results,omitempty"`
}

func Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var parsed params
	if response, failed := mcp.ParseArgs(req, args, &parsed); failed {
		return response
	}
	if parsed.Operation != "define" && parsed.Operation != "evaluate" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "operation must be 'define' or 'evaluate'", "Choose define to validate a contract or evaluate to calculate its verdict", mcp.WithParam("operation"))
	}
	if err := verification.ValidateContract(parsed.Contract); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid verification contract: "+err.Error(), "Supply schema_version '1', a contract_id, and complete unique assertions", mcp.WithParam("contract"))
	}
	if parsed.Operation == "define" {
		return mcp.Succeed(req, "Verification contract defined", map[string]any{
			"status":   "defined",
			"contract": parsed.Contract,
		})
	}
	result, err := verification.Evaluate(parsed.Contract, parsed.Results)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid verification results: "+err.Error(), "Use one valid result per contract assertion", mcp.WithParam("results"))
	}
	return mcp.Succeed(req, "Verification contract evaluated: "+string(result.Verdict), result)
}
