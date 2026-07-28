// Purpose: Tests for configure macro sequence save/replay.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_configure_sequence_test.go — Tests for macro sequence CRUD and replay.
package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/sequencehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

// newSequenceTestEnv creates a test env with an isolated session store
// so parallel tests don't interfere with each other's sequences.
func newSequenceTestEnv(t *testing.T) *toolTestEnv {
	t.Helper()
	env := newToolTestEnv(t)
	// Replace session store with one backed by t.TempDir for isolation
	store, err := persistence.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create isolated session store: %v", err)
	}
	t.Cleanup(func() { store.Shutdown() })
	env.handler.sessionStoreImpl = store
	env.handler.sequences = sequencehandler.New(sequencehandler.Deps{
		Store:          store,
		ReplayMu:       &replayMu,
		Interact:       env.handler.toolInteract,
		WaitForCommand: env.handler.capture.Queries().WaitForCommand,
		RecordAction:   env.handler.recordAIAction,
	})
	return env
}

// ============================================
// Save Sequence Tests
// ============================================

func TestSaveSequence_Valid(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "login-flow",
		"description": "Login to the app",
		"steps": [
			{"what": "navigate", "url": "https://example.com/login"},
			{"what": "click", "selector": "#submit"}
		]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "save_sequence", result)
	data := extractResultJSON(t, result)
	if data["status"] != "saved" {
		t.Errorf("expected status=saved, got %v", data["status"])
	}
	if data["name"] != "login-flow" {
		t.Errorf("expected name=login-flow, got %v", data["name"])
	}
	stepCount, _ := data["step_count"].(float64)
	if stepCount != 2 {
		t.Errorf("expected step_count=2, got %v", stepCount)
	}
}

func TestSaveSequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"steps": [{"what": "click", "selector": "#btn"}]
	}`)
	assertIsError(t, resp, "missing_param")
}

func TestSaveSequence_InvalidNameFormat(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "invalid name with spaces!",
		"steps": [{"what": "click", "selector": "#btn"}]
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_EmptySteps(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "empty-seq",
		"steps": []
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_TooManySteps(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	steps := make([]map[string]any, 51)
	for i := range steps {
		steps[i] = map[string]any{"what": "click", "selector": "#btn"}
	}
	argsMap := map[string]any{
		"what":  "save_sequence",
		"name":  "big-seq",
		"steps": steps,
	}
	argsJSON, _ := json.Marshal(argsMap)
	resp := callConfigureRaw(env.handler, string(argsJSON))
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_StepMissingAction(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "bad-step",
		"steps": [{"selector": "#btn"}]
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_Upsert(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	// Save initial
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "my-seq",
		"steps": [{"what": "click", "selector": "#a"}]
	}`)
	// Overwrite
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "my-seq",
		"steps": [
			{"what": "click", "selector": "#b"},
			{"what": "click", "selector": "#c"}
		]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "save_sequence upsert", result)
	data := extractResultJSON(t, result)
	stepCount, _ := data["step_count"].(float64)
	if stepCount != 2 {
		t.Errorf("expected upserted step_count=2, got %v", stepCount)
	}
}

func TestSaveSequence_WithTags(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "tagged-seq",
		"tags": ["auth", "setup"],
		"steps": [{"what": "navigate", "url": "https://example.com"}]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "save_sequence with tags", result)
}

// ============================================
// Get Sequence Tests
// ============================================

func TestGetSequence_Valid(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	// Save
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "get-test",
		"description": "A test sequence",
		"steps": [{"what": "navigate", "url": "https://example.com"}]
	}`)
	// Get
	resp := callConfigureRaw(env.handler, `{"what": "get_sequence", "name": "get-test"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "get_sequence", result)
	data := extractResultJSON(t, result)
	if data["name"] != "get-test" {
		t.Errorf("expected name=get-test, got %v", data["name"])
	}
	if data["description"] != "A test sequence" {
		t.Errorf("expected description, got %v", data["description"])
	}
	steps, ok := data["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Errorf("expected 1 step, got %v", data["steps"])
	}
}

func TestGetSequence_NotFound(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "get_sequence", "name": "nonexistent"}`)
	assertIsError(t, resp, "no_data")
}

func TestGetSequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "get_sequence"}`)
	assertIsError(t, resp, "missing_param")
}

// ============================================
// List Sequences Tests
// ============================================

func TestListSequences_Empty(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "list_sequences"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "list_sequences empty", result)
	data := extractResultJSON(t, result)
	sequences, ok := data["sequences"].([]any)
	if !ok {
		t.Fatalf("expected sequences array, got %T", data["sequences"])
	}
	if len(sequences) != 0 {
		t.Errorf("expected 0 sequences, got %d", len(sequences))
	}
}

func TestListSequences_Multiple(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	for _, name := range []string{"seq-a", "seq-b"} {
		argsMap := map[string]any{
			"what":  "save_sequence",
			"name":  name,
			"steps": []any{map[string]any{"what": "click", "selector": "#btn"}},
		}
		argsJSON, _ := json.Marshal(argsMap)
		callConfigureRaw(env.handler, string(argsJSON))
	}
	resp := callConfigureRaw(env.handler, `{"what": "list_sequences"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "list_sequences multiple", result)
	data := extractResultJSON(t, result)
	sequences, ok := data["sequences"].([]any)
	if !ok {
		t.Fatalf("expected sequences array, got %T", data["sequences"])
	}
	if len(sequences) != 2 {
		t.Errorf("expected 2 sequences, got %d", len(sequences))
	}
}

func TestListSequences_FilterByTags(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "auth-seq",
		"tags": ["auth"],
		"steps": [{"what": "click", "selector": "#btn"}]
	}`)
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "other-seq",
		"tags": ["other"],
		"steps": [{"what": "click", "selector": "#btn"}]
	}`)
	resp := callConfigureRaw(env.handler, `{"what": "list_sequences", "tags": ["auth"]}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "list_sequences filter", result)
	data := extractResultJSON(t, result)
	sequences, ok := data["sequences"].([]any)
	if !ok {
		t.Fatalf("expected sequences array, got %T", data["sequences"])
	}
	if len(sequences) != 1 {
		t.Errorf("expected 1 tagged sequence, got %d", len(sequences))
	}
}

// ============================================
// Delete Sequence Tests
// ============================================

func TestDeleteSequence_Valid(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "to-delete",
		"steps": [{"what": "click", "selector": "#btn"}]
	}`)
	resp := callConfigureRaw(env.handler, `{"what": "delete_sequence", "name": "to-delete"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "delete_sequence", result)
	data := extractResultJSON(t, result)
	if data["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", data["status"])
	}
	// Verify it's gone
	getResp := callConfigureRaw(env.handler, `{"what": "get_sequence", "name": "to-delete"}`)
	assertIsError(t, getResp, "no_data")
}

func TestDeleteSequence_NotFound(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "delete_sequence", "name": "nonexistent"}`)
	assertIsError(t, resp, "no_data")
}

func TestDeleteSequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "delete_sequence"}`)
	assertIsError(t, resp, "missing_param")
}

// ============================================
// Replay Sequence Tests (unit-level, no live browser)
// ============================================

func TestReplaySequence_NotFound(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "replay_sequence", "name": "nonexistent"}`)
	assertIsError(t, resp, "no_data")
}

func TestReplaySequence_MissingName(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{"what": "replay_sequence"}`)
	assertIsError(t, resp, "missing_param")
}

func TestReplaySequence_OverrideStepsLengthMismatch(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "two-step",
		"steps": [
			{"what": "click", "selector": "#a"},
			{"what": "click", "selector": "#b"}
		]
	}`)
	resp := callConfigureRaw(env.handler, `{
		"what": "replay_sequence",
		"name": "two-step",
		"override_steps": [null, null, null]
	}`)
	assertIsError(t, resp, "invalid_param")
}

func TestReplaySequence_QueuesPendingAsyncCommands(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, env.capture)

	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "one-exec-step",
		"steps": [{"what":"execute_js","script":"document.title"}]
	}`)

	resp := callConfigureRaw(env.handler, `{
		"what": "replay_sequence",
		"name": "one-exec-step",
		"step_timeout_ms": 50
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "replay_sequence queued", result)

	data := extractResultJSON(t, result)
	if status, _ := data["status"].(string); status != "queued" && status != "ok" && status != "partial" {
		t.Fatalf("status = %v, want queued|ok|partial", data["status"])
	}

	stepsQueued, ok := data["steps_queued"].(float64)
	if !ok {
		t.Fatalf("steps_queued type = %T, want number", data["steps_queued"])
	}
	if stepsQueued < 0 {
		t.Fatalf("steps_queued = %v, want non-negative", stepsQueued)
	}

	results, ok := data["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v, want exactly one step result", data["results"])
	}

	step0, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("results[0] type = %T, want map[string]any", results[0])
	}

	correlationID, _ := step0["correlation_id"].(string)
	if correlationID == "" {
		t.Fatal("step result should include correlation_id for pending async command")
	}
	if stepStatus, _ := step0["status"].(string); stepStatus != "queued" && stepStatus != "ok" && stepStatus != "error" {
		t.Fatalf("step status = %v, want queued|ok|error", step0["status"])
	}
}

func TestForceReplayAsyncInteractStep(t *testing.T) {
	t.Parallel()

	original := json.RawMessage(`{"action":"execute_js","script":"1+1","wait":true,"sync":true}`)
	mutated := replay.ForceAsync(original)

	var data map[string]any
	if err := json.Unmarshal(mutated, &data); err != nil {
		t.Fatalf("forceReplayAsyncInteractStep returned invalid JSON: %v", err)
	}
	if data["wait"] != false {
		t.Fatalf("wait = %v, want false", data["wait"])
	}
	if data["sync"] != false {
		t.Fatalf("sync = %v, want false", data["sync"])
	}
}

// ============================================
// Persistence Tests
// ============================================

func TestSequence_PersistsAcrossHandlerInstances(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	// Save a sequence
	callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "persist-test",
		"steps": [{"what": "navigate", "url": "https://example.com"}]
	}`)
	// Get with the same handler (verifies session store persistence)
	resp := callConfigureRaw(env.handler, `{"what": "get_sequence", "name": "persist-test"}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "persistence", result)
	data := extractResultJSON(t, result)
	if data["name"] != "persist-test" {
		t.Errorf("expected name=persist-test, got %v", data["name"])
	}
}

// ============================================
// Name Validation Edge Cases
// ============================================

func TestSaveSequence_NameTooLong(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	longName := ""
	for i := 0; i < 65; i++ {
		longName += "a"
	}
	argsMap := map[string]any{
		"what":  "save_sequence",
		"name":  longName,
		"steps": []any{map[string]any{"what": "click", "selector": "#btn"}},
	}
	argsJSON, _ := json.Marshal(argsMap)
	resp := callConfigureRaw(env.handler, string(argsJSON))
	assertIsError(t, resp, "invalid_param")
}

func TestSaveSequence_ValidNameChars(t *testing.T) {
	t.Parallel()
	env := newSequenceTestEnv(t)
	resp := callConfigureRaw(env.handler, `{
		"what": "save_sequence",
		"name": "valid-name_123",
		"steps": [{"what": "click", "selector": "#btn"}]
	}`)
	result := parseToolResult(t, resp)
	assertNonErrorResponse(t, "valid name chars", result)
}
