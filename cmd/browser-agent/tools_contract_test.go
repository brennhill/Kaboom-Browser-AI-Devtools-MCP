// Purpose: Tests for tool response contract compliance.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_contract_test.go — Response shape contracts for configure, generate, and interact tools.
// Lighter coverage than observe contracts — focuses on key actions that return JSON.
//
// Run: go test ./cmd/browser-agent -run "TestContract" -v
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
)

// ============================================
// Configure Tool Contracts
// ============================================

// configureContractEnv wraps configureTestEnv with contract assertions.
type configureContractEnv struct {
	*configureTestEnv
}

func newConfigureContractEnv(t *testing.T) *configureContractEnv {
	t.Helper()
	return &configureContractEnv{configureTestEnv: newConfigureTestEnv(t)}
}

func TestContractConfigure_Health(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"health"}`)
	if !ok {
		t.Fatal("configure health: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "health", data, []fieldSpec{
		required("server", "object"),
		required("buffers", "object"),
		required("memory", "object"),
	})
}

func TestContractConfigure_Clear(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"all"}`)
	if !ok {
		t.Fatal("configure clear: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "clear", data, []fieldSpec{
		required("status", "string"),
		required("buffer", "string"),
	})
}

func TestContractConfigure_NoiseRule_List(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("configure noise_rule list: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "noise_rule_list", data, []fieldSpec{
		required("rules", "array"),
		required("statistics", "object"),
	})
}

func TestContractConfigure_Streaming_Status(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"streaming","streaming_action":"status"}`)
	if !ok {
		t.Fatal("configure streaming status: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "streaming_status", data, []fieldSpec{
		required("config", "object"),
		required("notify_count", "number"),
		required("pending", "number"),
	})
}

func TestContractConfigure_UnknownAction_Error(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"invalid_action_xyz"}`)
	if !ok {
		t.Fatal("configure unknown action: no result")
	}
	assertStructuredError(t, "configure (unknown action)", result)
}

func TestContractConfigure_MissingAction_Error(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{}`)
	if !ok {
		t.Fatal("configure missing action: no result")
	}
	assertStructuredError(t, "configure (missing action)", result)
}

// ============================================
// Generate Tool Contracts
// ============================================

// generateContractEnv wraps generateTestEnv with contract assertions.
type generateContractEnv struct {
	*generateTestEnv
}

func newGenerateContractEnv(t *testing.T) *generateContractEnv {
	t.Helper()
	return &generateContractEnv{generateTestEnv: newGenerateTestEnv(t)}
}

func TestContractGenerate_Reproduction(t *testing.T) {
	env := newGenerateContractEnv(t)
	// Seed actions so reproduction has data to generate from
	env.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000, URL: "https://example.com", ToURL: "https://example.com"},
		{Type: "click", Timestamp: 2000, URL: "https://example.com", Selectors: map[string]any{"text": "Go"}},
	})
	result, ok := env.callGenerate(t, `{"what":"reproduction"}`)
	if !ok {
		t.Fatal("generate reproduction: no result")
	}
	// Reproduction returns text content, not JSON
	assertNonErrorResponse(t, "reproduction", result)
}

func TestContractGenerate_Test(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"test"}`)
	if !ok {
		t.Fatal("generate test: no result")
	}
	assertNonErrorResponse(t, "test", result)
}

func TestContractGenerate_PRSummary(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"pr_summary"}`)
	if !ok {
		t.Fatal("generate pr_summary: no result")
	}
	assertNonErrorResponse(t, "pr_summary", result)
}

func TestContractGenerate_HAR(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"har"}`)
	if !ok {
		t.Fatal("generate har: no result")
	}
	assertNonErrorResponse(t, "har", result)
}

func TestContractGenerate_CSP(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"csp"}`)
	if !ok {
		t.Fatal("generate csp: no result")
	}
	assertNonErrorResponse(t, "csp", result)
}

func TestContractGenerate_SRI(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"sri"}`)
	if !ok {
		t.Fatal("generate sri: no result")
	}
	assertNonErrorResponse(t, "sri", result)
}

func TestContractGenerate_SARIF(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"sarif"}`)
	if !ok {
		t.Fatal("generate sarif: no result")
	}
	assertNonErrorResponse(t, "sarif", result)
}

func TestContractGenerate_UnknownFormat_Error(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"what":"invalid_format_xyz"}`)
	if !ok {
		t.Fatal("generate unknown format: no result")
	}
	assertStructuredError(t, "generate (unknown format)", result)
}

func TestContractGenerate_MissingFormat_Error(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{}`)
	if !ok {
		t.Fatal("generate missing format: no result")
	}
	assertStructuredError(t, "generate (missing format)", result)
}

// ============================================
// Interact Tool Contracts
// ============================================

// interactContractEnv wraps interactTestEnv with contract assertions.
type interactContractEnv struct {
	*interactTestEnv
}

func newInteractContractEnv(t *testing.T) *interactContractEnv {
	t.Helper()
	return &interactContractEnv{interactTestEnv: newInteractTestEnv(t)}
}

func TestContractInteract_ListStates(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"list_states"}`)
	if !ok {
		t.Fatal("interact list_states: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "list_states", data, []fieldSpec{
		required("states", "array"),
		required("count", "number"),
	})
}

func TestContractInteract_SaveState_MissingName(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"save_state"}`)
	if !ok {
		t.Fatal("interact save_state: no result")
	}
	assertStructuredError(t, "save_state (missing snapshot_name)", result)
}

func TestContractInteract_UnknownAction_Error(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"invalid_action_xyz"}`)
	if !ok {
		t.Fatal("interact unknown action: no result")
	}
	assertStructuredError(t, "interact (unknown action)", result)
}

func TestContractInteract_MissingAction_Error(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{}`)
	if !ok {
		t.Fatal("interact missing action: no result")
	}
	assertStructuredError(t, "interact (missing action)", result)
}

// ============================================
// Bad Path Contracts — Invalid JSON
// ============================================

func TestContractBadPath_Observe_InvalidJSON(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserveWithArgs(t, `{invalid json`)
	if !ok {
		t.Fatal("observe invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "observe (invalid JSON)", result, "invalid_json")
}

func TestContractBadPath_Configure_InvalidJSON(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{invalid json`)
	if !ok {
		t.Fatal("configure invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "configure (invalid JSON)", result, "invalid_json")
}

func TestContractBadPath_Generate_InvalidJSON(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{invalid json`)
	if !ok {
		t.Fatal("generate invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "generate (invalid JSON)", result, "invalid_json")
}

func TestContractBadPath_Interact_InvalidJSON(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{invalid json`)
	if !ok {
		t.Fatal("interact invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "interact (invalid JSON)", result, "invalid_json")
}

// ============================================
// Bad Path Contracts — Missing Required Params
// ============================================

// TestContractBadPath_Configure_QueryDOM_MissingSelector removed in Phase 0: moved to analyze({what:'dom'})

func TestContractBadPath_Configure_TestBoundary_MissingTestID(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"test_boundary_start"}`)
	if !ok {
		t.Fatal("configure test_boundary_start: no result")
	}
	assertStructuredErrorCode(t, "test_boundary_start (missing test_id)", result, "missing_param")
}

func TestContractBadPath_Configure_NoiseRule_Remove_MissingRuleID(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"remove"}`)
	if !ok {
		t.Fatal("configure noise_rule remove: no result")
	}
	assertStructuredErrorCode(t, "noise_rule remove (missing rule_id)", result, "missing_param")
}

func TestContractBadPath_Interact_Highlight_MissingSelector(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"highlight"}`)
	if !ok {
		t.Fatal("interact highlight: no result")
	}
	assertStructuredErrorCode(t, "highlight (missing selector)", result, "missing_param")
}

func TestContractBadPath_Interact_ExecuteJS_MissingScript(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"execute_js"}`)
	if !ok {
		t.Fatal("interact execute_js: no result")
	}
	assertStructuredErrorCode(t, "execute_js (missing script)", result, "missing_param")
}

func TestContractBadPath_Interact_Navigate_MissingURL(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"navigate"}`)
	if !ok {
		t.Fatal("interact navigate: no result")
	}
	assertStructuredErrorCode(t, "navigate (missing url)", result, "missing_param")
}

func TestContractBadPath_Interact_LoadState_MissingName(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"load_state"}`)
	if !ok {
		t.Fatal("interact load_state: no result")
	}
	assertStructuredErrorCode(t, "load_state (missing snapshot_name)", result, "missing_param")
}

func TestContractBadPath_Interact_DeleteState_MissingName(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"delete_state"}`)
	if !ok {
		t.Fatal("interact delete_state: no result")
	}
	assertStructuredErrorCode(t, "delete_state (missing snapshot_name)", result, "missing_param")
}

// ============================================
// Bad Path Contracts — Invalid Param Values
// ============================================

func TestContractBadPath_Configure_Clear_InvalidBuffer(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"nonexistent_buffer"}`)
	if !ok {
		t.Fatal("configure clear invalid buffer: no result")
	}
	assertStructuredErrorCode(t, "clear (invalid buffer)", result, "invalid_param")
}

func TestContractBadPath_Configure_NoiseRule_UnknownAction(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"invalid_xyz"}`)
	if !ok {
		t.Fatal("configure noise_rule unknown action: no result")
	}
	assertStructuredErrorCode(t, "noise_rule (unknown noise_action)", result, "unknown_mode")
}
