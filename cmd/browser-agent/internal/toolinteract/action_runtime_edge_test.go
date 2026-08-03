// action_runtime_edge_test.go — Tests retry-policy edge and terminal contracts.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestSecureJitterStaysWithinRequestedRange(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		got := secureJitter(7)
		if got < 0 || got >= 7 {
			t.Fatalf("secureJitter(7) = %d", got)
		}
	}
	if got := secureJitter(0); got != 0 {
		t.Fatalf("secureJitter(0) = %d", got)
	}
}

func TestRetryContractStopsUnchangedSecondAttempt(t *testing.T) {
	runtime := NewActionRuntime(RuntimeDeps{})
	firstArgs := json.RawMessage(`{"what":"click","selector":"#save"}`)
	runtime.armRetryContract("first", "", firstArgs)
	runtime.armRetryContract("second", "click", json.RawMessage(`{"selector":"#save","correlation_id":"first"}`))

	data := map[string]any{"error_code": "element_not_found", "effective_url": "https://example.test"}
	decision := runtime.AttachRetryContext("second", data, "error", "")
	if !decision.Terminal || decision.Cause != "strategy_not_changed" {
		t.Fatalf("decision = %+v", decision)
	}
	if data["terminal"] != true || data["retryable"] != false || data["evidence_summary"] == nil {
		t.Fatalf("terminal response = %#v", data)
	}
	context, ok := data["retry_context"].(map[string]any)
	if !ok || context["attempt"] != 2 || context["policy_violation"] != "strategy_unchanged" {
		t.Fatalf("retry context = %#v", data["retry_context"])
	}
}

func TestRetryContractMissingParentAndAttemptLimit(t *testing.T) {
	runtime := NewActionRuntime(RuntimeDeps{})
	runtime.armRetryContract("orphan", "click", json.RawMessage(`{"element_id":"x","correlation_id":"missing"}`))
	orphan, _ := runtime.getRetryState("orphan")
	if orphan.Attempt != 2 || orphan.PolicyViolation != "parent_context_missing" {
		t.Fatalf("orphan = %+v", orphan)
	}

	runtime.armRetryContract("third", "click", json.RawMessage(`{"scope_selector":"main","correlation_id":"orphan"}`))
	third, _ := runtime.getRetryState("third")
	if third.Attempt != maxRetryAttemptsPerStep || third.PolicyViolation != "attempt_limit_exceeded" {
		t.Fatalf("third = %+v", third)
	}
	data := map[string]any{"error": " timeout "}
	decision := runtime.AttachRetryContext("third", data, "timeout", "")
	if !decision.Terminal || decision.Cause != "max_attempts_reached" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestRetryHelpersCoverStrategiesAndPruning(t *testing.T) {
	for _, tc := range []struct {
		args string
		want string
	}{
		{args: `{}`, want: "default"},
		{args: `{"element_id":"e"}`, want: "element_handle"},
		{args: `{"scope_rect":{"x":1}}`, want: "scoped_selector"},
		{args: `{"frame":"main"}`, want: "frame_targeted"},
		{args: `{"selector":"button"}`, want: "selector"},
		{args: `{"index":1}`, want: "indexed"},
		{args: `{"world":"isolated"}`, want: "world_switch"},
	} {
		strategy, fingerprint := deriveRetryStrategy("click", json.RawMessage(tc.args))
		if strategy != tc.want || !strings.Contains(fingerprint, `"action":"click"`) {
			t.Fatalf("%s => %q %q", tc.args, strategy, fingerprint)
		}
	}
	if stableMarshalForRetry(nil) != "" {
		t.Fatal("nil retry map was marshaled")
	}

	runtime := NewActionRuntime(RuntimeDeps{})
	runtime.retryByCommand = map[string]*commandRetryState{
		"old": {CreatedAt: time.Unix(1, 0)},
		"new": {CreatedAt: time.Unix(2, 0)},
	}
	runtime.pruneRetryStatesLocked(1)
	if _, exists := runtime.retryByCommand["old"]; exists {
		t.Fatal("old retry state was not pruned")
	}
	if decision := runtime.AttachRetryContext("missing", map[string]any{}, "error", "fallback"); decision.Terminal {
		t.Fatal("missing retry state became terminal")
	}
	if decision := (*ActionRuntime)(nil).AttachRetryContext("x", map[string]any{}, "error", "fallback"); decision.Terminal {
		t.Fatal("nil runtime became terminal")
	}
}

func TestQueuedResponseRecognitionEdgeCases(t *testing.T) {
	queued := mcp.Succeed(testReq(), "queued", map[string]any{"status": "queued"})
	if !isResponseQueued(queued) {
		t.Fatal("queued response not recognized")
	}
	for _, response := range []mcp.JSONRPCResponse{
		{},
		{Result: json.RawMessage(`bad`)},
		{Result: mustRuntimeJSON(t, mcp.MCPToolResult{})},
		mcp.SucceedText(testReq(), "not json"),
		mcp.Succeed(testReq(), "complete", map[string]any{"status": "complete"}),
	} {
		if isResponseQueued(response) {
			t.Fatalf("false queued response: %s", response.Result)
		}
	}
}

func mustRuntimeJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
