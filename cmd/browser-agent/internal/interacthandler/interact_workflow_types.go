// Purpose: Defines shared workflow aliases/helpers for interact compound actions.
// Why: Keeps workflow glue code centralized across navigate/form/a11y workflow handlers.

package interacthandler

import (
	"time"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// WorkflowStep — type alias delegated to internal/tools/interact package.
type WorkflowStep = act.WorkflowStep

// FormField — type alias delegated to internal/tools/interact package.
type FormField = act.FormField

// isErrorResponse — delegated to internal/tools/interact package.
func isErrorResponse(resp JSONRPCResponse) bool {
	return act.IsErrorResponse(resp)
}

// isNonFinalResponse — delegated to internal/tools/interact package.
func isNonFinalResponse(resp JSONRPCResponse) bool {
	return act.IsNonFinalResponse(resp)
}

// responseStatus — delegated to internal/tools/interact package.
func responseStatus(resp JSONRPCResponse) string {
	return act.ResponseStatus(resp)
}

// workflowResult — delegated to internal/tools/interact package.
func workflowResult(req JSONRPCRequest, workflow string, trace []WorkflowStep, lastResp JSONRPCResponse, start time.Time) JSONRPCResponse {
	return act.WorkflowResult(req, workflow, trace, lastResp, start)
}
