// batch_test.go — Deterministic contracts for inline interaction batches.
// Docs: docs/features/feature/interact-explore/index.md

package interactbatch

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type batchFixture struct {
	handler  *Handler
	store    *capture.Capture
	actions  []string
	recorded int
}

func newBatchFixture(t *testing.T) *batchFixture {
	t.Helper()
	fixture := &batchFixture{store: capture.NewCapture()}
	t.Cleanup(fixture.store.Close)
	var handler *Handler
	handler = New(Deps{
		RequirePilot:     allow,
		RequireExtension: allow,
		Capture:          func() *capture.Capture { return fixture.store },
		RecordAIAction:   func(string, string, map[string]any) { fixture.recorded++ },
		ReplayMu:         &sync.Mutex{},
		Now:              deterministicBatchClock(),
		Interact: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			var step struct {
				What string `json:"what"`
			}
			if err := json.Unmarshal(args, &step); err != nil {
				return mcp.Fail(req, mcp.ErrInvalidJSON, "invalid step", "fix JSON")
			}
			fixture.actions = append(fixture.actions, step.What)
			switch step.What {
			case "batch":
				return handler.Handle(req, args)
			case "click":
				return mcp.Fail(req, mcp.ErrMissingParam, "selector required", "add selector")
			default:
				return mcp.SucceedText(req, "ok")
			}
		},
	})
	fixture.handler = handler
	return fixture
}

func allow(mcp.JSONRPCRequest, ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}

func deterministicBatchClock() func() time.Time {
	current := time.Unix(1_700_000_000, 0)
	return func() time.Time {
		result := current
		current = current.Add(time.Millisecond)
		return result
	}
}

func batchRequest() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, ClientID: "batch-test"}
}

func batchData(t *testing.T, response mcp.JSONRPCResponse) (mcp.MCPToolResult, map[string]any) {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("batch response has no content")
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	if start < 0 {
		return result, nil
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text[start:]), &data); err != nil {
		t.Fatalf("decode batch data: %v", err)
	}
	return result, data
}

func TestHandleValidatesBatchContractBeforeExecution(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want string
	}{
		{"missing steps", `{}`, "non-empty"},
		{"empty steps", `{"steps":[]}`, "non-empty"},
		{"step missing action", `{"steps":[{"text":"hello"}]}`, "what"},
		{"obsolete action selector", `{"steps":[{"action":"subtitle"}]}`, "what"},
		{"malformed JSON", `{bad`, mcp.ErrInvalidJSON},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBatchFixture(t)
			result, _ := batchData(t, fixture.handler.Handle(batchRequest(), json.RawMessage(testCase.args)))
			if !result.IsError || !strings.Contains(strings.ToLower(result.Content[0].Text), strings.ToLower(testCase.want)) {
				t.Fatalf("result = %+v, want error containing %q", result, testCase.want)
			}
			if len(fixture.actions) != 0 || fixture.recorded != 0 {
				t.Fatalf("invalid batch mutated state: actions=%v recorded=%d", fixture.actions, fixture.recorded)
			}
		})
	}
}

func TestHandleEnforcesMaximumStepCount(t *testing.T) {
	t.Parallel()
	steps := make([]map[string]string, maxSteps+1)
	for index := range steps {
		steps[index] = map[string]string{"what": "subtitle"}
	}
	encoded, err := json.Marshal(map[string]any{"steps": steps})
	if err != nil {
		t.Fatal(err)
	}
	fixture := newBatchFixture(t)
	result, _ := batchData(t, fixture.handler.Handle(batchRequest(), encoded))
	if !result.IsError || !strings.Contains(result.Content[0].Text, "50") {
		t.Fatalf("oversized result = %+v", result)
	}
}

func TestHandleExecutionStatusAndCounters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, args, status string
		executed, failed   float64
	}{
		{"success", `{"steps":[{"what":"subtitle"}]}`, "ok", 1, 0},
		{"mixed", `{"steps":[{"what":"subtitle"},{"what":"click"},{"what":"subtitle"}]}`, "partial", 3, 1},
		{"all failed", `{"steps":[{"what":"click"},{"what":"click"}]}`, "error", 2, 2},
		{"stop on error", `{"steps":[{"what":"click"},{"what":"subtitle"}],"continue_on_error":false}`, "error", 1, 1},
		{"stop after step", `{"steps":[{"what":"subtitle"},{"what":"subtitle"}],"stop_after_step":1}`, "ok", 1, 0},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBatchFixture(t)
			result, data := batchData(t, fixture.handler.Handle(batchRequest(), json.RawMessage(testCase.args)))
			if result.IsError || data["status"] != testCase.status || data["steps_executed"] != testCase.executed || data["steps_failed"] != testCase.failed {
				t.Fatalf("batch result=%+v data=%#v", result, data)
			}
			if data["steps_queued"].(float64) > data["steps_executed"].(float64) || data["steps_failed"].(float64) > data["steps_executed"].(float64) {
				t.Fatalf("counter invariant failed: %#v", data)
			}
			if fixture.recorded != 1 {
				t.Fatalf("audit records = %d, want 1", fixture.recorded)
			}
		})
	}
}

func TestHandleAggregatesCompletedExtensionCommand(t *testing.T) {
	t.Parallel()
	store := capture.NewCapture()
	t.Cleanup(store.Close)
	handler := New(Deps{
		RequirePilot: allow, RequireExtension: allow,
		Capture:        func() *capture.Capture { return store },
		ReplayMu:       &sync.Mutex{},
		RecordAIAction: func(string, string, map[string]any) {},
		Now:            deterministicBatchClock(),
		Interact: func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			store.Queries().RegisterCommand("step-1", "", time.Second)
			store.Queries().ApplyCommandResult("step-1", "complete", json.RawMessage(`{"success":true}`), "")
			return mcp.Succeed(req, "queued", map[string]any{"correlation_id": "step-1"})
		},
	})
	result, data := batchData(t, handler.Handle(batchRequest(), json.RawMessage(`{"steps":[{"what":"click","selector":"#save"}]}`)))
	if result.IsError || data["status"] != "ok" || data["steps_executed"] != float64(1) || data["steps_failed"] != float64(0) {
		t.Fatalf("completed extension batch result=%+v data=%#v", result, data)
	}
	results, ok := data["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("step results = %#v", data["results"])
	}
	step := results[0].(map[string]any)
	if step["correlation_id"] != "step-1" || step["status"] != "ok" {
		t.Fatalf("completed step = %#v", step)
	}
}

func TestHandleRejectsNestedOrConcurrentBatch(t *testing.T) {
	t.Parallel()
	fixture := newBatchFixture(t)
	result, data := batchData(t, fixture.handler.Handle(batchRequest(), json.RawMessage(`{"steps":[{"what":"batch","steps":[{"what":"subtitle"}]}]}`)))
	if result.IsError || data["status"] != "error" || data["steps_failed"] != float64(1) {
		t.Fatalf("nested batch result=%+v data=%#v", result, data)
	}

	locked := &sync.Mutex{}
	locked.Lock()
	handler := New(Deps{
		RequirePilot: allow, RequireExtension: allow, ReplayMu: locked,
		Interact: func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return mcp.SucceedText(req, "ok")
		},
		RecordAIAction: func(string, string, map[string]any) {},
	})
	blocked, _ := batchData(t, handler.Handle(batchRequest(), json.RawMessage(`{"steps":[{"what":"subtitle"}]}`)))
	if !blocked.IsError || !strings.Contains(blocked.Content[0].Text, "currently executing") {
		t.Fatalf("concurrent result = %+v", blocked)
	}
}

func TestStripComposableScreenshot(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{"what":"type","text":"hello","include_screenshot":false}`)
	output := stripComposableScreenshot(input)
	var data map[string]any
	if err := json.Unmarshal(output, &data); err != nil {
		t.Fatalf("decode stripped step: %v", err)
	}
	if _, exists := data["include_screenshot"]; exists || data["what"] != "type" || data["text"] != "hello" {
		t.Fatalf("stripped step = %#v", data)
	}
	invalid := json.RawMessage(`{bad`)
	if got := stripComposableScreenshot(invalid); string(got) != string(invalid) {
		t.Fatalf("malformed step changed: %s", got)
	}
}
