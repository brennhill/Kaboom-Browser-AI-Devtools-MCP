// Purpose: Generates Playwright test scripts from captured browser actions and context (error, interaction, regression).
// Why: Converts runtime telemetry into executable test artifacts for reproduction and regression coverage.
// Docs: docs/features/feature/test-generation/index.md

package testgenhandler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
)

// ============================================
// MCP Entry Point: test_from_context
// ============================================

// testGenContextDispatch maps context values to their generator functions.
var testGenContextDispatch = map[string]func(h *Handler, params testgen.TestFromContextRequest) (*testgen.GeneratedTest, error){
	"error":       (*Handler).generateTestFromError,
	"interaction": (*Handler).generateTestFromInteraction,
	"regression":  (*Handler).generateTestFromRegression,
}

// testGenErrorMapping type for MCP error responses.
type testGenErrorMapping struct {
	code    string
	message string
	retry   string
	hint    string
}

var testGenErrorMappings []testGenErrorMapping

func init() {
	for _, m := range testgen.ErrorMappings {
		testGenErrorMappings = append(testGenErrorMappings, testGenErrorMapping{
			code: m.Code, message: m.Message, retry: m.Retry, hint: m.Hint,
		})
	}
}

func (h *Handler) HandleGenerateTestFromContext(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params testgen.TestFromContextRequest

	warnings, err := mcp.UnmarshalWithWarnings(args, &params)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	warnings = toolgenerate.FilterGenerateDispatchWarnings(warnings)

	if errResp, blocked := validateTestFromContextParams(req, params); blocked {
		return errResp
	}

	if params.Framework == "" {
		params.Framework = "playwright"
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "inline"
	}

	generator := testGenContextDispatch[params.Context]
	generatedTest, err := generator(h, params)
	if err != nil {
		return testGenErrorToResponse(req.ID, err)
	}

	summary := fmt.Sprintf("Generated %s test '%s' (%d assertions)",
		generatedTest.Framework,
		generatedTest.Filename,
		generatedTest.Assertions)

	data := map[string]any{
		"test":     generatedTest,
		"metadata": generatedTest.Metadata,
	}

	resp := mcp.Succeed(req, summary, data)

	return mcp.AppendWarningsToResponse(resp, warnings)
}

var validTestGenContexts = []string{"error", "interaction", "regression"}

func validateTestFromContextParams(req mcp.JSONRPCRequest, params testgen.TestFromContextRequest) (mcp.JSONRPCResponse, bool) {
	if resp, blocked := toolresp.RequireString(req, params.Context, "context", "Add the 'context' parameter and call again"); blocked {
		return resp, true
	}
	if resp, blocked := toolresp.RequireOneOf(req, params.Context, "context", validTestGenContexts, "Use a valid context value"); blocked {
		return resp, true
	}
	return mcp.JSONRPCResponse{}, false
}

func testGenErrorToResponse(reqID any, err error) mcp.JSONRPCResponse {
	errStr := err.Error()
	for _, m := range testGenErrorMappings {
		if strings.Contains(errStr, m.code) {
			return mcp.JSONRPCResponse{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      reqID,
				Result:  mcp.StructuredErrorResponse(m.code, m.message, m.retry, mcp.WithHint(m.hint)),
			}
		}
	}
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      reqID,
		Result:  mcp.StructuredErrorResponse(mcp.ErrInternal, "Failed to generate test: "+err.Error(), "Check the input parameters and ensure captured data is available, then retry"),
	}
}
