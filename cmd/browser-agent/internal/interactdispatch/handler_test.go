// handler_test.go — Interact routing and composable result orchestration contracts.
// Docs: docs/features/feature/interact-explore/index.md

package interactdispatch

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type dispatchFixture struct {
	actions     []string
	jittered    []string
	sideEffects []string
	delays      []time.Duration
}

func newDispatchFixture(t *testing.T) (*Handler, *dispatchFixture) {
	t.Helper()
	fixture := &dispatchFixture{}
	action := func(name string) Action {
		return func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			fixture.actions = append(fixture.actions, name)
			return mcp.Succeed(req, name, map[string]any{"action": name})
		}
	}
	appendResult := func(name string) func(mcp.JSONRPCResponse, mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return func(resp mcp.JSONRPCResponse, _ mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			fixture.sideEffects = append(fixture.sideEffects, name)
			return resp
		}
	}
	handler := New(Deps{
		Actions: map[string]Action{"click": action("click"), "navigate": action("navigate"), "subtitle": action("subtitle")},
		ApplyJitter: func(name string) int {
			fixture.jittered = append(fixture.jittered, name)
			return 0
		},
		QueueSubtitle: func(_ mcp.JSONRPCRequest, value string) {
			fixture.sideEffects = append(fixture.sideEffects, "subtitle:"+value)
		},
		QueueAutoDismiss: func(mcp.JSONRPCRequest) { fixture.sideEffects = append(fixture.sideEffects, "auto_dismiss") },
		QueueWaitForStable: func(_ mcp.JSONRPCRequest, ms int) {
			fixture.sideEffects = append(fixture.sideEffects, "stable:"+strconv.Itoa(ms))
		},
		QueueActionDiff:   func(mcp.JSONRPCRequest) { fixture.sideEffects = append(fixture.sideEffects, "action_diff") },
		AppendScreenshot:  appendResult("screenshot"),
		AppendInteractive: appendResult("interactive"),
		Delay:             func(duration time.Duration) { fixture.delays = append(fixture.delays, duration) },
	})
	return handler, fixture
}

func dispatchReq() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: float64(1), ClientID: "test-client"}
}

func resultText(t *testing.T, resp mcp.JSONRPCResponse) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

func TestHandlerDispatchesCanonicalActionAndJitter(t *testing.T) {
	h, fixture := newDispatchFixture(t)
	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`))
	if text := resultText(t, resp); !strings.Contains(text, `"action":"click"`) {
		t.Fatalf("response = %s", text)
	}
	if strings.Join(fixture.actions, ",") != "click" || strings.Join(fixture.jittered, ",") != "click" {
		t.Fatalf("actions=%v jittered=%v", fixture.actions, fixture.jittered)
	}
}

func TestHandlerRejectsInvalidEvidenceBeforeAction(t *testing.T) {
	h, fixture := newDispatchFixture(t)
	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click","evidence":"sometimes"}`))
	if !strings.Contains(resultText(t, resp), mcp.ErrInvalidParam) || len(fixture.actions) != 0 {
		t.Fatalf("response=%s actions=%v", resultText(t, resp), fixture.actions)
	}
}

func TestHandlerRunsOnlyCompatibleComposableEffects(t *testing.T) {
	h, fixture := newDispatchFixture(t)
	args := json.RawMessage(`{"what":"navigate","subtitle":"Loading","auto_dismiss":true,"wait_for_stable":true,"stability_ms":250,"action_diff":true,"include_screenshot":true,"include_interactive":true}`)
	resp := h.Handle(dispatchReq(), args)
	if resultText(t, resp) == "" {
		t.Fatal("empty response")
	}
	want := []string{"subtitle:Loading", "auto_dismiss", "stable:250", "action_diff", "screenshot", "interactive"}
	if strings.Join(fixture.sideEffects, ",") != strings.Join(want, ",") {
		t.Fatalf("side effects = %v, want %v", fixture.sideEffects, want)
	}
	if len(fixture.delays) != 1 || fixture.delays[0] != composableSideEffectDelay {
		t.Fatalf("delays = %v", fixture.delays)
	}
}

func TestHandlerDoesNotRecurseSubtitleOrDecorateErrors(t *testing.T) {
	h, fixture := newDispatchFixture(t)
	h.Handle(dispatchReq(), json.RawMessage(`{"what":"subtitle","subtitle":"again"}`))
	if len(fixture.sideEffects) != 0 {
		t.Fatalf("subtitle recursion side effects = %v", fixture.sideEffects)
	}

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"missing","subtitle":"bad","include_screenshot":true,"include_interactive":true,"action_diff":true}`))
	if !strings.Contains(resultText(t, resp), mcp.ErrUnknownMode) || len(fixture.sideEffects) != 0 {
		t.Fatalf("error response=%s side effects=%v", resultText(t, resp), fixture.sideEffects)
	}
}

func TestHandlerCopiesActionMap(t *testing.T) {
	actions := map[string]Action{"click": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse { return mcp.SucceedText(req, "ok") }}
	h := New(Deps{Actions: actions})
	delete(actions, "click")
	if text := resultText(t, h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`))); text != "ok" {
		t.Fatalf("copied action response = %s", text)
	}
}

func TestHandlerPreservesUnknownAndCanonicalAsyncFields(t *testing.T) {
	var captured json.RawMessage
	h := New(Deps{Actions: map[string]Action{
		"click": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			captured = append(json.RawMessage(nil), args...)
			return mcp.SucceedText(req, "ok")
		},
	}})
	for _, input := range []string{
		`{"what":"click","async":true}`,
		`{"what":"click","background":false}`,
	} {
		h.Handle(dispatchReq(), json.RawMessage(input))
		var parsed map[string]any
		if err := json.Unmarshal(captured, &parsed); err != nil {
			t.Fatalf("decode captured args: %v", err)
		}
		if strings.Contains(input, `"async"`) && parsed["async"] != true {
			t.Fatalf("async field rewritten: %#v", parsed)
		}
		if strings.Contains(input, `"background"`) && parsed["background"] != false {
			t.Fatalf("background field rewritten: %#v", parsed)
		}
	}
}

func TestHandlerActionNamesAreSortedAndDefensive(t *testing.T) {
	h := New(Deps{Actions: map[string]Action{"z": nil, "a": nil}})
	names := h.ActionNames()
	if strings.Join(names, ",") != "a,z" {
		t.Fatalf("action names = %v", names)
	}
	names[0] = "changed"
	if strings.Join(h.ActionNames(), ",") != "a,z" {
		t.Fatal("caller mutated action-name surface")
	}
}
