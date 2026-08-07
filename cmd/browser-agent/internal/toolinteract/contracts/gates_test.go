// gates_test.go — Cross-owner interact guard ordering contracts.
// Docs: docs/features/feature/interact-explore/index.md

package contracts_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

type gateFixture struct {
	capture *capture.Capture
	browser *toolinteract.BrowserActions
	dom     *toolinteract.DOMActions
	queued  []queries.PendingQuery
}

func newGateFixture() *gateFixture {
	fixture := &gateFixture{capture: capture.NewCapture()}
	capturefixture.SetPilot(fixture.capture, false)
	guards := toolguard.New(fixture.capture, context.Background(), time.Millisecond)
	runtime := toolinteract.NewActionRuntime(toolinteract.RuntimeDeps{
		RequireCSPClear: guards.RequireCSPClear,
		EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
			fixture.queued = append(fixture.queued, query)
			return mcp.JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, _ json.RawMessage, summary string) mcp.JSONRPCResponse {
			return mcp.Succeed(req, summary, map[string]any{"status": "queued", "correlation_id": correlationID})
		},
		RecordAIAction: func(string, string, map[string]any) {},
	})
	fixture.dom = toolinteract.NewDOMActions(runtime, toolinteract.DOMDeps{
		RequirePilot: guards.RequirePilot, RequireExtension: guards.RequireExtension, RequireTabTracking: guards.RequireTabTracking,
		RecordDOMPrimitiveAction: func(string, string, string, string) {},
	})
	fixture.browser = toolinteract.NewBrowserActions(runtime, nil, toolinteract.BrowserDeps{
		RequirePilot: guards.RequirePilot, RequireExtension: guards.RequireExtension, RequireTabTracking: guards.RequireTabTracking,
		Capture: func() *capture.Capture { return fixture.capture }, InjectCSPBlockedActions: func(response mcp.JSONRPCResponse) mcp.JSONRPCResponse { return response },
	})
	return fixture
}

func errorCode(t *testing.T, response mcp.JSONRPCResponse) (string, string) {
	t.Helper()
	got := result(t, response)
	if !got.IsError || len(got.Content) == 0 {
		t.Fatalf("expected structured error, got %#v", got)
	}
	start := strings.IndexByte(got.Content[0].Text, '{')
	var structured mcp.StructuredError
	if start < 0 || json.Unmarshal([]byte(got.Content[0].Text[start:]), &structured) != nil {
		t.Fatalf("missing structured error: %#v", got)
	}
	return structured.ErrorCode, structured.Message
}

func TestExecuteJSGatesFireInCanonicalOrder(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(*gateFixture)
		args  string
		code  string
		text  string
	}{
		{name: "parameters", args: `{"world":"main"}`, code: mcp.ErrMissingParam, text: "script"},
		{name: "pilot", args: `{"script":"1","world":"main"}`, code: mcp.ErrCodePilotDisabled, text: "Pilot"},
		{name: "extension", setup: enablePilot, args: `{"script":"1","world":"main"}`, code: mcp.ErrNoData, text: "Extension"},
		{name: "tracking", setup: connect, args: `{"script":"1","world":"main"}`, code: mcp.ErrNoData, text: "tab"},
		{name: "CSP", setup: trackAndRestrict, args: `{"script":"1","world":"main"}`, code: mcp.ErrExtError, text: "CSP"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGateFixture()
			if testCase.setup != nil {
				testCase.setup(fixture)
			}
			code, message := errorCode(t, fixture.browser.Handle("execute_js", request(), json.RawMessage(testCase.args)))
			if code != testCase.code || !strings.Contains(message, testCase.text) || len(fixture.queued) != 0 {
				t.Fatalf("code=%q message=%q queued=%#v", code, message, fixture.queued)
			}
		})
	}
}

func enablePilot(fixture *gateFixture) { capturefixture.SetPilot(fixture.capture, true) }

func connect(fixture *gateFixture) {
	enablePilot(fixture)
	capturefixture.Connect(fixture.capture)
}

func trackAndRestrict(fixture *gateFixture) {
	connect(fixture)
	capturefixture.Track(fixture.capture, 42, "https://example.test")
	capturefixture.SetCSP(fixture.capture, true, "script_exec")
}

func TestActionFamiliesApplyOnlyTheirRequiredGates(t *testing.T) {
	fixture := newGateFixture()
	if got := result(t, fixture.browser.Handle("subtitle", request(), json.RawMessage(`{"text":"hello"}`))); got.IsError {
		t.Fatalf("subtitle required browser gates: %#v", got)
	}

	connect(fixture)
	for _, action := range []struct{ name, args string }{
		{name: "navigate", args: `{"url":"https://example.test"}`},
		{name: "switch_tab", args: `{"tab_id":42}`},
	} {
		if got := result(t, fixture.browser.Handle(action.name, request(), json.RawMessage(action.args))); got.IsError {
			t.Fatalf("%s required prior tracking: %#v", action.name, got)
		}
	}

	code, message := errorCode(t, fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(`{"selector":"#save"}`), "click"))
	if code != mcp.ErrNoData || !strings.Contains(message, "tab") {
		t.Fatalf("click gate = %q %q", code, message)
	}
}
