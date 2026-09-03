// Purpose: Defines structured error codes, ToolError type, and error response builders for MCP tool results.
// Docs: docs/features/feature/query-service/index.md

package mcp

import (
	"encoding/json"
	"fmt"
)

// Error codes are self-describing snake_case strings.
// Every code tells the LLM what went wrong.
const (
	// Input errors — LLM can fix arguments and retry immediately
	ErrInvalidJSON    = "invalid_json"
	ErrMissingParam   = "missing_param"
	ErrInvalidParam   = "invalid_param"
	ErrUnknownMode    = "unknown_mode"
	ErrPathNotAllowed = "path_not_allowed"

	// State errors — LLM must change state before retrying
	ErrNotInitialized       = "not_initialized"
	ErrNoData               = "no_data"
	ErrCodePilotDisabled    = "pilot_disabled"
	ErrAlreadyRecording     = "already_recording"
	ErrOsAutomationDisabled = "os_automation_disabled"
	ErrRateLimited          = "rate_limited"
	ErrCursorExpired        = "cursor_expired"
	// ErrOriginNotConsented: the user has not permitted kaboom to DRIVE this origin.
	// Not retryable — retrying without a grant produces the same refusal.
	ErrOriginNotConsented = "origin_not_consented"

	// Communication errors — retry with backoff
	ErrExtTimeout = "extension_timeout"
	ErrExtError   = "extension_error"
	ErrQueueFull  = "queue_full"

	// Internal errors — do not retry
	ErrInternal      = "internal_error"
	ErrMarshalFailed = "marshal_failed"
	ErrExportFailed  = "export_failed"
)

// StructuredError is embedded in MCP text content. Every field is
// self-describing so an LLM can act on it without a lookup table.
type StructuredError struct {
	// Canonical contract fields.
	ErrorCode        string `json:"error_code"`
	Message          string `json:"message"`
	RecoveryPlaybook string `json:"recovery_playbook"`

	Retryable    bool   `json:"retryable"`
	RetryAfterMs int    `json:"retry_after_ms,omitempty"`
	Final        bool   `json:"final,omitempty"`
	Param        string `json:"param,omitempty"`
	Hint         string `json:"hint,omitempty"`
	Action       string `json:"action,omitempty"`
	Selector     string `json:"selector,omitempty"`

	// RecoveryToolCall is a copy-pasteable MCP tool call the LLM can use to recover.
	// Keys: "tool" (string), "arguments" (map[string]any).
	RecoveryToolCall map[string]any `json:"recovery_tool_call,omitempty"`
}

// StructuredErrorResponse constructs an MCP error response. Format:
//
//	Error: missing_param — Add the 'what' parameter and call again
//	{"error_code":"missing_param","message":"...","recovery_playbook":"Add the 'what' parameter and call again",...}
//
// The retry string is a plain-English instruction the LLM can follow directly.
func StructuredErrorResponse(code, message, recoveryPlaybook string, opts ...func(*StructuredError)) json.RawMessage {
	se := StructuredError{
		ErrorCode:        code,
		Message:          message,
		RecoveryPlaybook: recoveryPlaybook,
	}
	// Apply retryable defaults based on error code first, then user opts can override
	for _, defaultOpt := range retryDefaultsForCode(code) {
		defaultOpt(&se)
	}
	for _, opt := range opts {
		opt(&se)
	}

	// Error impossible: StructuredError is a simple struct with no circular refs or unsupported types
	seJSON, _ := json.Marshal(se)
	text := fmt.Sprintf("Error: %s — %s\n%s", se.ErrorCode, se.RecoveryPlaybook, string(seJSON))

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
		IsError: true,
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

// WithParam is an option function to add param field to StructuredError.
func WithParam(p string) func(*StructuredError) {
	return func(se *StructuredError) { se.Param = p }
}

// WithHint is an option function to add hint field to StructuredError.
func WithHint(h string) func(*StructuredError) {
	return func(se *StructuredError) { se.Hint = h }
}

// WithAction sets the action that triggered the error.
func WithAction(a string) func(*StructuredError) {
	return func(se *StructuredError) { se.Action = a }
}

// WithSelector sets the selector that was being targeted when the error occurred.
func WithSelector(s string) func(*StructuredError) {
	return func(se *StructuredError) { se.Selector = s }
}

// WithRetryable marks whether the error is retryable by the LLM.
func WithRetryable(retryable bool) func(*StructuredError) {
	return func(se *StructuredError) { se.Retryable = retryable }
}

// WithRetryAfterMs sets the suggested delay before retrying (milliseconds).
func WithRetryAfterMs(ms int) func(*StructuredError) {
	return func(se *StructuredError) { se.RetryAfterMs = ms }
}

// WithFinal marks a structured error as terminal/non-terminal for async command flows.
func WithFinal(final bool) func(*StructuredError) {
	return func(se *StructuredError) { se.Final = final }
}

// WithRecoveryToolCall attaches a copy-pasteable MCP tool call for LLM recovery.
// toolCall should have keys "tool" (string) and "arguments" (map[string]any).
func WithRecoveryToolCall(toolCall map[string]any) func(*StructuredError) {
	return func(se *StructuredError) { se.RecoveryToolCall = toolCall }
}

// retryDefaultsForCode returns option functions that set retryable and retry_after_ms
// based on the error code. Retryable errors are transient conditions the LLM can
// retry after a brief delay; non-retryable errors require the LLM to change its input.
func retryDefaultsForCode(code string) []func(*StructuredError) {
	switch code {
	case ErrExtTimeout:
		return []func(*StructuredError){WithRetryable(true), WithRetryAfterMs(1000)}
	case ErrExtError:
		return []func(*StructuredError){WithRetryable(true), WithRetryAfterMs(2000)}
	case ErrRateLimited:
		return []func(*StructuredError){WithRetryable(true), WithRetryAfterMs(1000)}
	case ErrCursorExpired:
		return []func(*StructuredError){WithRetryable(true), WithRetryAfterMs(500)}
	case ErrNoData:
		return []func(*StructuredError){WithRetryable(true), WithRetryAfterMs(2000)}
	default:
		return []func(*StructuredError){WithRetryable(false)}
	}
}
