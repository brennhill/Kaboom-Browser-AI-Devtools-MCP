// rich_action_test.go — External contracts for rich DOM command forwarding.
// Docs: docs/features/feature/interact-explore/index.md

package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestDOMRichArgumentsReachCanonicalPendingQuery(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name       string
		args       json.RawMessage
		wantKey    string
		wantValue  any
		wantAbsent string
	}{
		{name: "analysis enabled", args: json.RawMessage(`{"selector":"#submit","analyze":true}`), wantKey: "analyze", wantValue: true},
		{name: "analysis omitted", args: json.RawMessage(`{"selector":"#submit"}`), wantAbsent: "analyze"},
		{name: "frame selector", args: json.RawMessage(`{"selector":"#submit","frame":"iframe.payment"}`), wantKey: "frame", wantValue: "iframe.payment"},
		{name: "frame index", args: json.RawMessage(`{"selector":"#submit","frame":0}`), wantKey: "frame", wantValue: float64(0)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var queued queries.PendingQuery
			runtime := toolinteract.NewActionRuntime(toolinteract.RuntimeDeps{
				EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
					queued = query
					return mcp.JSONRPCResponse{}, false
				},
				MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, _ json.RawMessage, summary string) mcp.JSONRPCResponse {
					return mcp.Succeed(req, summary, map[string]any{"correlation_id": correlationID})
				},
				RecordAIAction: func(string, string, map[string]any) {},
			})
			actions := toolinteract.NewDOMActions(runtime, toolinteract.DOMDeps{
				RequirePilot: allow, RequireExtension: allow, RequireTabTracking: allow,
				RecordDOMPrimitiveAction: func(string, string, string, string) {},
			})
			response := actions.HandleDOMPrimitive(request(), testCase.args, "click")
			if result(t, response).IsError || queued.Type != "dom_action" || !strings.HasPrefix(queued.CorrelationID, "dom_click_") {
				t.Fatalf("response=%#v query=%#v", response, queued)
			}
			var params map[string]any
			if err := json.Unmarshal(queued.Params, &params); err != nil {
				t.Fatal(err)
			}
			if testCase.wantAbsent != "" {
				if _, exists := params[testCase.wantAbsent]; exists {
					t.Fatalf("unexpected %s in %#v", testCase.wantAbsent, params)
				}
			} else if params[testCase.wantKey] != testCase.wantValue {
				t.Fatalf("%s = %#v, want %#v", testCase.wantKey, params[testCase.wantKey], testCase.wantValue)
			}
		})
	}
}
