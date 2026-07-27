// artifacts_test_impl.go — Implements generate(test) artifact assembly.
// Why: Keeps test script generation isolated from other generate artifact formats.

package toolgenerate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	gen "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/generate"
)

// HandleGenerateTest generates a Playwright test from captured browser actions.
func HandleGenerateTest(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params gen.TestGenParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if params.TestName == "" {
		params.TestName = "generated test"
	}

	allActions := d.GetCapture().GetAllEnhancedActions()
	actions := gen.FilterLastN(allActions, params.LastN)
	script := gen.GenerateTestScript(actions, params)

	result := map[string]any{
		"script":       script,
		"test_name":    params.TestName,
		"action_count": len(actions),
		"metadata": map[string]any{
			"generated_at":      time.Now().Format(time.RFC3339),
			"actions_available": len(allActions),
			"actions_included":  len(actions),
			"assert_network":    params.AssertNetwork,
			"assert_no_errors":  params.AssertNoErrors,
		},
	}

	if len(actions) == 0 {
		result["reason"] = "no_actions_captured"
		result["hint"] = "Navigate and interact with the browser first, then call generate(test) again."
	}

	summary := fmt.Sprintf("Playwright test '%s' (%d actions)", params.TestName, len(actions))
	return mcp.Succeed(req, summary, result)
}
