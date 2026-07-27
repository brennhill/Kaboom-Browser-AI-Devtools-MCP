// Purpose: Re-exports MCP error codes and structured error option wrappers.
// Why: Gives all tool handlers a uniform error vocabulary without importing internal/mcp directly.

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Error code aliases — all callers in package main use these unchanged.
const (
	ErrInvalidJSON          = mcp.ErrInvalidJSON
	ErrMissingParam         = mcp.ErrMissingParam
	ErrInvalidParam         = mcp.ErrInvalidParam
	ErrUnknownMode          = mcp.ErrUnknownMode
	ErrPathNotAllowed       = mcp.ErrPathNotAllowed
	ErrNotInitialized       = mcp.ErrNotInitialized
	ErrNoData               = mcp.ErrNoData
	ErrCodePilotDisabled    = mcp.ErrCodePilotDisabled
	ErrOsAutomationDisabled = mcp.ErrOsAutomationDisabled
	ErrRateLimited          = mcp.ErrRateLimited
	ErrCursorExpired        = mcp.ErrCursorExpired
	ErrExtTimeout           = mcp.ErrExtTimeout
	ErrExtError             = mcp.ErrExtError
	ErrQueueFull            = mcp.ErrQueueFull
	ErrInternal             = mcp.ErrInternal
	ErrMarshalFailed        = mcp.ErrMarshalFailed
	ErrExportFailed         = mcp.ErrExportFailed
)

// StructuredError alias.
type StructuredError = mcp.StructuredError

func mcpStructuredError(code, message, retry string, opts ...func(*StructuredError)) json.RawMessage {
	return mcp.StructuredErrorResponse(code, message, retry, opts...)
}

func withParam(p string) func(*StructuredError)    { return mcp.WithParam(p) }
func withHint(h string) func(*StructuredError)     { return mcp.WithHint(h) }
func withAction(a string) func(*StructuredError)   { return mcp.WithAction(a) }
func withSelector(s string) func(*StructuredError) { return mcp.WithSelector(s) }
func withRetryable(retryable bool) func(*StructuredError) {
	return mcp.WithRetryable(retryable)
}
func withRetryAfterMs(ms int) func(*StructuredError) { return mcp.WithRetryAfterMs(ms) }
func withFinal(final bool) func(*StructuredError)    { return mcp.WithFinal(final) }
func withRecoveryToolCall(toolCall map[string]any) func(*StructuredError) {
	return mcp.WithRecoveryToolCall(toolCall)
}

// describeCapabilitiesRecovery returns a recovery_tool_call for describe_capabilities scoped to a tool.
func describeCapabilitiesRecovery(toolName string) func(*StructuredError) {
	return withRecoveryToolCall(map[string]any{
		"tool": "configure",
		"arguments": map[string]any{
			"what": "describe_capabilities",
			"tool": toolName,
		},
	})
}

func appendCanonicalWhatAliasWarning(resp JSONRPCResponse, aliasParam, mode, deprecatedIn, removeIn string) JSONRPCResponse {
	if strings.TrimSpace(aliasParam) == "" || strings.TrimSpace(mode) == "" {
		return resp
	}
	var warning string
	if deprecatedIn != "" && removeIn != "" {
		warning = fmt.Sprintf("Parameter '%s' is deprecated (since %s, removal planned %s); use what=%q.", aliasParam, deprecatedIn, removeIn, mode)
	} else if deprecatedIn != "" {
		warning = fmt.Sprintf("Parameter '%s' is deprecated (since %s); use what=%q.", aliasParam, deprecatedIn, mode)
	} else {
		warning = fmt.Sprintf("Accepted alias parameter '%s'; canonical parameter is 'what' (use what=%q).", aliasParam, mode)
	}
	return appendWarningsToResponse(resp, []string{warning})
}

func whatAliasConflictResponse(req JSONRPCRequest, aliasParam, whatValue, aliasValue, validValues string) JSONRPCResponse {
	hint := "Use only 'what' when specifying tool mode/action."
	if strings.TrimSpace(validValues) != "" {
		hint += " Valid values: " + validValues
	}
	return fail(req, ErrInvalidParam,
		fmt.Sprintf("Conflicting parameters: what=%q and %s=%q", whatValue, aliasParam, aliasValue),
		"Send only the canonical 'what' parameter and retry.",
		withParam("what"), withHint(hint),
	)
}
