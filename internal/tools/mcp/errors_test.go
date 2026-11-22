// errors_test.go — Tests canonical structured MCP error serialization.

package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func structuredErrorFromResult(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError || len(result.Content) == 0 {
		t.Fatalf("result is not a structured error: %#v", result)
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	if start < 0 {
		t.Fatalf("structured JSON missing: %q", result.Content[0].Text)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text[start:]), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestStructuredErrorSerializesRetryAndRecoveryContract(t *testing.T) {
	t.Parallel()
	retryable := structuredErrorFromResult(t, StructuredErrorResponse(
		ErrExtTimeout, "Extension timed out", "Retry the command", WithRetryable(true), WithRetryAfterMs(1000),
	))
	if retryable["retryable"] != true || retryable["retry_after_ms"] != float64(1000) {
		t.Fatalf("retry contract = %#v", retryable)
	}
	canonical := structuredErrorFromResult(t, StructuredErrorResponse(
		ErrMissingParam, "Missing parameter", "Call interact with what=list_interactive",
	))
	if canonical["error_code"] != ErrMissingParam || canonical["recovery_playbook"] != "Call interact with what=list_interactive" {
		t.Fatalf("canonical contract = %#v", canonical)
	}
	for _, legacy := range []string{"error", "retry"} {
		if _, exists := canonical[legacy]; exists {
			t.Errorf("legacy field %q retained: %#v", legacy, canonical)
		}
	}
}

func TestEveryCanonicalErrorSerializesRequiredRecoveryFields(t *testing.T) {
	t.Parallel()
	for _, code := range []string{
		ErrInvalidJSON,
		ErrMissingParam,
		ErrInvalidParam,
		ErrUnknownMode,
		ErrExtTimeout,
		ErrExtError,
		ErrInternal,
		ErrNoData,
		ErrRateLimited,
		ErrNotInitialized,
		ErrCursorExpired,
		ErrMarshalFailed,
	} {
		t.Run(code, func(t *testing.T) {
			decoded := structuredErrorFromResult(t, StructuredErrorResponse(code, "contract failure", "follow recovery steps"))
			if decoded["error_code"] != code {
				t.Fatalf("error_code = %#v, want %q", decoded["error_code"], code)
			}
			if decoded["recovery_playbook"] != "follow recovery steps" {
				t.Fatalf("recovery_playbook = %#v", decoded["recovery_playbook"])
			}
			if _, exists := decoded["retryable"]; !exists {
				t.Fatalf("retryable missing: %#v", decoded)
			}
		})
	}
}

func TestStructuredErrorOmitsUnsetContext(t *testing.T) {
	t.Parallel()
	plain := structuredErrorFromResult(t, StructuredErrorResponse(ErrInternal, "Internal", "Do not retry"))
	if plain["retryable"] != false {
		t.Fatalf("default retryable = %#v", plain["retryable"])
	}
	for _, omitted := range []string{"retry_after_ms", "action", "selector"} {
		if _, exists := plain[omitted]; exists {
			t.Errorf("unset field %q retained: %#v", omitted, plain)
		}
	}
	contextual := structuredErrorFromResult(t, StructuredErrorResponse(
		ErrNoData, "Not connected", "Check extension", WithAction("click"), WithSelector("#submit-btn"),
	))
	if contextual["action"] != "click" || contextual["selector"] != "#submit-btn" {
		t.Fatalf("error context = %#v", contextual)
	}
}
