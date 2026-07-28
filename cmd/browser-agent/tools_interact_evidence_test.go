// Purpose: Tests for interact action evidence capture.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_evidence_test.go — TDD tests for interact evidence capture mode.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func setEvidenceCaptureForTest(t *testing.T, capture func(*toolinteract.Deps, string) toolinteract.EvidenceShot) {
	t.Helper()
	toolinteract.SetEvidenceCaptureFn(capture)
	t.Cleanup(toolinteract.ResetEvidenceCaptureFn)
}

func TestCommandResult_EvidenceAlwaysIncludesBeforeAfterPaths(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	calls := 0
	shots := []toolinteract.EvidenceShot{
		{Path: "/tmp/evidence-before.png"},
		{Path: "/tmp/evidence-after.png"},
	}
	idx := 0
	setEvidenceCaptureForTest(t, func(_ *toolinteract.Deps, _ string) toolinteract.EvidenceShot {
		calls++
		if idx >= len(shots) {
			return toolinteract.EvidenceShot{Error: "unexpected_extra_capture"}
		}
		shot := shots[idx]
		idx++
		return shot
	})

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true,"evidence":"always"}`)
	if !ok || result.IsError {
		t.Fatalf("click should queue successfully, got: %s", firstText(result))
	}

	queued := extractResultJSON(t, result)
	corrID, _ := queued["correlation_id"].(string)
	if corrID == "" {
		t.Fatalf("correlation_id missing in queued response: %v", queued)
	}

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true}`), "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)
	observe := parseToolResult(t, resp)
	if observe.IsError {
		t.Fatalf("observe command_result should be success, got: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	evidence, ok := data["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence payload missing: %v", data["evidence"])
	}
	if evidence["before"] != "/tmp/evidence-before.png" {
		t.Fatalf("evidence.before = %v, want /tmp/evidence-before.png", evidence["before"])
	}
	if evidence["after"] != "/tmp/evidence-after.png" {
		t.Fatalf("evidence.after = %v, want /tmp/evidence-after.png", evidence["after"])
	}
	if partial, _ := evidence["partial"].(bool); partial {
		t.Fatalf("evidence.partial should be false for successful before+after captures, got %v", evidence["partial"])
	}
	if calls != 2 {
		t.Fatalf("capture calls = %d, want 2 (before+after)", calls)
	}
}

func TestCommandResult_EvidenceOnMutationSkipsReadOnlyAction(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	calls := 0
	setEvidenceCaptureForTest(t, func(_ *toolinteract.Deps, _ string) toolinteract.EvidenceShot {
		calls++
		return toolinteract.EvidenceShot{Path: "/tmp/should-not-capture.png"}
	})

	result, ok := env.callInteract(t, `{"what":"get_text","selector":"h1","background":true,"evidence":"on_mutation"}`)
	if !ok || result.IsError {
		t.Fatalf("get_text should queue successfully, got: %s", firstText(result))
	}

	queued := extractResultJSON(t, result)
	corrID, _ := queued["correlation_id"].(string)
	if corrID == "" {
		t.Fatalf("correlation_id missing in queued response: %v", queued)
	}

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"value":"headline"}`), "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)
	observe := parseToolResult(t, resp)
	if observe.IsError {
		t.Fatalf("observe command_result should be success, got: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	evidenceRaw, hasEvidence := data["evidence"]
	if !hasEvidence {
		t.Fatal("evidence should be included when caller explicitly sets evidence mode")
	}
	evidence, ok := evidenceRaw.(map[string]any)
	if !ok {
		t.Fatalf("evidence payload should be an object, got %T", evidenceRaw)
	}
	if _, exists := evidence["before"]; exists {
		t.Fatalf("evidence.before should not be present for read-only action in on_mutation mode: %v", evidence["before"])
	}
	if _, exists := evidence["after"]; exists {
		t.Fatalf("evidence.after should not be present for read-only action in on_mutation mode: %v", evidence["after"])
	}
	if calls != 0 {
		t.Fatalf("capture calls = %d, want 0 for read-only action in on_mutation mode", calls)
	}
}

func TestCommandResult_EvidencePartialWhenAfterCaptureFails(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	calls := 0
	shots := []toolinteract.EvidenceShot{
		{Path: "/tmp/evidence-before.png"},
		{Error: "screenshot_timeout"},
		{Error: "screenshot_timeout"},
	}
	idx := 0
	setEvidenceCaptureForTest(t, func(_ *toolinteract.Deps, _ string) toolinteract.EvidenceShot {
		calls++
		if idx >= len(shots) {
			return toolinteract.EvidenceShot{Error: "unexpected_extra_capture"}
		}
		shot := shots[idx]
		idx++
		return shot
	})

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true,"evidence":"always"}`)
	if !ok || result.IsError {
		t.Fatalf("click should queue successfully, got: %s", firstText(result))
	}

	queued := extractResultJSON(t, result)
	corrID, _ := queued["correlation_id"].(string)
	if corrID == "" {
		t.Fatalf("correlation_id missing in queued response: %v", queued)
	}

	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(`{"success":false,"error":"element_not_found"}`), "element_not_found")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)
	observe := parseToolResult(t, resp)
	if !observe.IsError {
		t.Fatalf("observe command_result should be error for failed action, got: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	evidence, ok := data["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence payload missing: %v", data["evidence"])
	}
	if evidence["before"] != "/tmp/evidence-before.png" {
		t.Fatalf("evidence.before = %v, want /tmp/evidence-before.png", evidence["before"])
	}
	if _, exists := evidence["after"]; exists {
		t.Fatalf("evidence.after should be absent when after-capture fails, got %v", evidence["after"])
	}
	if partial, _ := evidence["partial"].(bool); !partial {
		t.Fatalf("evidence.partial should be true when one capture fails, got %v", evidence["partial"])
	}
	errorsMap, ok := evidence["errors"].(map[string]any)
	if !ok {
		t.Fatalf("evidence.errors missing for partial capture: %v", evidence["errors"])
	}
	if afterErr, _ := errorsMap["after"].(string); afterErr == "" {
		t.Fatalf("evidence.errors.after missing: %v", errorsMap)
	}
	if calls < 3 {
		t.Fatalf("capture calls = %d, want >=3 (before + deterministic retry for failed after capture)", calls)
	}
}

func TestInteractEvidence_InvalidModeReturnsError(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","evidence":"sometimes"}`)
	if !ok {
		t.Fatal("interact should return a structured error result")
	}
	if !result.IsError {
		t.Fatalf("invalid evidence mode should return isError=true, got: %s", firstText(result))
	}
	text := strings.ToLower(firstText(result))
	if !strings.Contains(text, "evidence") || !strings.Contains(text, "invalid") {
		t.Fatalf("expected invalid evidence error details, got: %s", firstText(result))
	}
}
