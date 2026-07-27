// Purpose: Validate interact(what="batch") multi-step interaction execution.
// Why: Prevents regressions in batch step execution, error handling, and continue_on_error behavior.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_batch_test.go — Tests for interact(what="batch") mode.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsInteractBatch_Dispatches(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Batch with valid steps should not return "unknown action" error
	resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"subtitle","text":"hello"}],"step_timeout_ms":100,"sync":false}`)
	result := parseToolResult(t, resp)

	// Should not be "unknown action" error
	if result.IsError {
		text := firstText(result)
		if strings.Contains(text, "Unknown interact action") {
			t.Fatalf("batch should be a known action, got: %s", text)
		}
	}
}

// setupBatchHandler creates a ToolHandler with pilot/extension enabled,
// which is required since handleBatch has early requirePilot/requireExtension gates (#9.R3.9).
func setupBatchHandler(t *testing.T) *ToolHandler {
	t.Helper()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	return h
}

func TestToolsInteractBatch_MissingSteps(t *testing.T) {
	t.Parallel()
	h := setupBatchHandler(t)

	resp := callInteractRaw(h, `{"what":"batch","sync":false}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("batch without steps should be an error")
	}
	text := firstText(result)
	if !strings.Contains(text, "steps") && !strings.Contains(text, "Steps") {
		t.Errorf("error should mention 'steps', got: %s", text)
	}
}

func TestToolsInteractBatch_EmptySteps(t *testing.T) {
	t.Parallel()
	h := setupBatchHandler(t)

	resp := callInteractRaw(h, `{"what":"batch","steps":[],"sync":false}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("batch with empty steps should be an error")
	}
}

func TestToolsInteractBatch_TooManySteps(t *testing.T) {
	t.Parallel()
	h := setupBatchHandler(t)

	// Build 51 steps
	steps := make([]map[string]string, 51)
	for i := range steps {
		steps[i] = map[string]string{"what": "subtitle", "text": "step"}
	}
	stepsJSON, _ := json.Marshal(steps)
	argsJSON := `{"what":"batch","steps":` + string(stepsJSON) + `,"sync":false}`

	resp := callInteractRaw(h, argsJSON)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("batch with >50 steps should be an error")
	}
	text := firstText(result)
	if !strings.Contains(text, "50") {
		t.Errorf("error should mention max step limit, got: %s", text)
	}
}

func TestToolsInteractBatch_StepMissingWhat(t *testing.T) {
	t.Parallel()
	h := setupBatchHandler(t)

	resp := callInteractRaw(h, `{"what":"batch","steps":[{"text":"hello"}],"sync":false}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("step without 'what' should be an error")
	}
	text := firstText(result)
	if !strings.Contains(text, "what") {
		t.Errorf("error should mention missing 'what' field, got: %s", text)
	}
}

// ============================================
// Schema Tests
// ============================================

func TestToolsInteractSchema_BatchInWhatEnum(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'what' property")
	}
	enumValues, ok := whatProp["enum"].([]string)
	if !ok {
		t.Fatal("'what' property missing enum")
	}

	found := false
	for _, v := range enumValues {
		if v == "batch" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'batch', got: %v", enumValues)
	}
}

func TestToolsInteractSchema_StepsParam(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	stepsProp, ok := props["steps"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'steps' property")
	}
	if stepsProp["type"] != "array" {
		t.Errorf("steps type = %v, want 'array'", stepsProp["type"])
	}
}

// ============================================
// Execution Tests (sequential — shares global replayMu)
// ============================================

func TestToolsInteractBatch_Execution(t *testing.T) {
	// These subtests run sequentially because they contend on the global replayMu.
	h := setupBatchHandler(t)

	t.Run("ResponseStructure", func(t *testing.T) {
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"subtitle","text":"test"}],"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		if len(result.Content) == 0 {
			t.Fatal("batch should return at least one content block")
		}

		assertSnakeCaseFields(t, string(resp.Result))
	})

	t.Run("ResponseFieldCompleteness", func(t *testing.T) {
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"subtitle","text":"test"}],"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		requiredFields := []string{"status", "steps_executed", "steps_failed", "steps_queued", "steps_total", "duration_ms", "results", "message"}
		for _, field := range requiredFields {
			if _, ok := data[field]; !ok {
				t.Errorf("batch response missing required field %q", field)
			}
		}
	})

	t.Run("ContinueOnErrorDefault", func(t *testing.T) {
		// Two steps: first is invalid (missing selector for click), second is valid subtitle.
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"click"},{"what":"subtitle","text":"hi"}],"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		stepsExecuted, _ := data["steps_executed"].(float64)
		if stepsExecuted < 2 {
			t.Errorf("continue_on_error=true (default) should execute all steps, executed=%v", stepsExecuted)
		}
	})

	t.Run("ContinueOnErrorFalse", func(t *testing.T) {
		// Two steps: first is invalid (missing selector for click), second is valid subtitle.
		// With continue_on_error=false, should stop after the first error.
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"click"},{"what":"subtitle","text":"hi"}],"step_timeout_ms":100,"continue_on_error":false,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		stepsExecuted, _ := data["steps_executed"].(float64)
		if stepsExecuted != 1 {
			t.Errorf("continue_on_error=false should stop after first error, executed=%v", stepsExecuted)
		}
		status, _ := data["status"].(string)
		if status != "error" {
			t.Errorf("continue_on_error=false with error should have status 'error', got %q", status)
		}
	})

	t.Run("AllStepsFailedStatus", func(t *testing.T) {
		// All steps fail — status should be "error", not "partial"
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"click"},{"what":"click"}],"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		status, _ := data["status"].(string)
		if status != "error" {
			t.Errorf("all-steps-failed should have status 'error', got %q", status)
		}
		stepsFailed, _ := data["steps_failed"].(float64)
		stepsExecuted, _ := data["steps_executed"].(float64)
		if stepsFailed != stepsExecuted {
			t.Errorf("steps_failed (%v) should equal steps_executed (%v) when all fail", stepsFailed, stepsExecuted)
		}
	})

	t.Run("NestedBatchReturnsError", func(t *testing.T) {
		// A batch step containing another batch should fail (batch acquires replayMu, nested would deadlock).
		// Since replayMu is already held by the outer batch, the inner batch step will get an error.
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"batch","steps":[{"what":"subtitle","text":"nested"}]}],"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		stepsFailed, _ := data["steps_failed"].(float64)
		if stepsFailed < 1 {
			t.Error("nested batch step should fail (replayMu already held)")
		}
	})

	t.Run("StopAfterStep", func(t *testing.T) {
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"subtitle","text":"a"},{"what":"subtitle","text":"b"},{"what":"subtitle","text":"c"}],"stop_after_step":1,"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		stepsExecuted, _ := data["steps_executed"].(float64)
		if stepsExecuted != 1 {
			t.Errorf("stop_after_step=1 should execute exactly 1 step, got %v", stepsExecuted)
		}
	})

	t.Run("StopAfterStepWithFailure", func(t *testing.T) {
		// step 1 fails (click with no selector), stop_after_step=2 + continue_on_error=true
		// Should execute stop_after_step steps, not stop early due to error
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"click"},{"what":"subtitle","text":"b"},{"what":"subtitle","text":"c"}],"stop_after_step":2,"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		stepsExecuted, _ := data["steps_executed"].(float64)
		if stepsExecuted != 2 {
			t.Errorf("stop_after_step=2 with continue_on_error=true should execute 2 steps, got %v", stepsExecuted)
		}
		stepsFailed, _ := data["steps_failed"].(float64)
		if stepsFailed != 1 {
			t.Errorf("expected 1 failed step, got %v", stepsFailed)
		}
	})

	t.Run("CounterInvariant", func(t *testing.T) {
		// Verify stepsFailed <= stepsExecuted invariant across various scenarios
		scenarios := []string{
			`{"what":"batch","steps":[{"what":"subtitle","text":"ok"}],"step_timeout_ms":100,"sync":false}`,
			`{"what":"batch","steps":[{"what":"click"},{"what":"click"}],"step_timeout_ms":100,"sync":false}`,
			`{"what":"batch","steps":[{"what":"click"},{"what":"subtitle","text":"ok"}],"step_timeout_ms":100,"sync":false}`,
			`{"what":"batch","steps":[{"what":"click"}],"step_timeout_ms":100,"continue_on_error":false,"sync":false}`,
		}
		for i, args := range scenarios {
			resp := callInteractRaw(h, args)
			result := parseToolResult(t, resp)
			data := extractResultJSON(t, result)
			failed, _ := data["steps_failed"].(float64)
			executed, _ := data["steps_executed"].(float64)
			total, _ := data["steps_total"].(float64)
			if failed > executed {
				t.Errorf("scenario %d: steps_failed (%v) > steps_executed (%v)", i, failed, executed)
			}
			if executed > total {
				t.Errorf("scenario %d: steps_executed (%v) > steps_total (%v)", i, executed, total)
			}
		}
	})

	t.Run("MixedResultsPartialStatus", func(t *testing.T) {
		// 3 steps: 1 fails (click), 2 succeed (subtitle) — should be "partial"
		resp := callInteractRaw(h, `{"what":"batch","steps":[{"what":"subtitle","text":"a"},{"what":"click"},{"what":"subtitle","text":"c"}],"step_timeout_ms":100,"sync":false}`)
		result := parseToolResult(t, resp)

		data := extractResultJSON(t, result)
		status, _ := data["status"].(string)
		if status != "partial" {
			t.Errorf("mixed results (some pass, some fail) should be 'partial', got %q", status)
		}
		stepsFailed, _ := data["steps_failed"].(float64)
		if stepsFailed != 1 {
			t.Errorf("expected 1 failed step, got %v", stepsFailed)
		}
		stepsExecuted, _ := data["steps_executed"].(float64)
		if stepsExecuted != 3 {
			t.Errorf("expected 3 executed steps, got %v", stepsExecuted)
		}
	})
}

// ============================================
// stripComposableScreenshotFromStep (#9.R12)
// ============================================

func TestStripComposableScreenshotFromStep_RemovesFlag(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"what":"click","selector":"btn","include_screenshot":true}`)
	output := toolinteract.StripComposableScreenshotFromStep(input)

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if _, has := result["include_screenshot"]; has {
		t.Error("include_screenshot should be removed")
	}
	if result["what"] != "click" {
		t.Errorf("what should be preserved, got %v", result["what"])
	}
	if result["selector"] != "btn" {
		t.Errorf("selector should be preserved, got %v", result["selector"])
	}
}

func TestStripComposableScreenshotFromStep_NoOp(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"what":"click","selector":"btn"}`)
	output := toolinteract.StripComposableScreenshotFromStep(input)

	if string(output) != string(input) {
		t.Errorf("output should be unchanged when no include_screenshot present\ninput:  %s\noutput: %s", input, output)
	}
}

func TestStripComposableScreenshotFromStep_InvalidJSON(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{invalid json`)
	output := toolinteract.StripComposableScreenshotFromStep(input)

	if string(output) != string(input) {
		t.Errorf("invalid JSON should be returned unchanged\ninput:  %s\noutput: %s", input, output)
	}
}

func TestStripComposableScreenshotFromStep_PreservesOtherFields(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"what":"type","selector":"input","text":"hello","clear":true,"include_screenshot":true}`)
	output := toolinteract.StripComposableScreenshotFromStep(input)

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if _, has := result["include_screenshot"]; has {
		t.Error("include_screenshot should be removed")
	}
	// All other fields should survive
	if result["what"] != "type" {
		t.Errorf("what = %v, want 'type'", result["what"])
	}
	if result["selector"] != "input" {
		t.Errorf("selector = %v, want 'input'", result["selector"])
	}
	if result["text"] != "hello" {
		t.Errorf("text = %v, want 'hello'", result["text"])
	}
	if result["clear"] != true {
		t.Errorf("clear = %v, want true", result["clear"])
	}
}

func TestStripComposableScreenshotFromStep_FalseValue(t *testing.T) {
	t.Parallel()
	// Even include_screenshot:false should be stripped (it's wasteful to send)
	input := json.RawMessage(`{"what":"click","include_screenshot":false}`)
	output := toolinteract.StripComposableScreenshotFromStep(input)

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if _, has := result["include_screenshot"]; has {
		t.Error("include_screenshot:false should also be stripped")
	}
}
