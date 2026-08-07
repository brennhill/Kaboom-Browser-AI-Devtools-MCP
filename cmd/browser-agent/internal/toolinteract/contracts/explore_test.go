// explore_test.go — External contracts for the explore-page action owner.
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

type exploreFixture struct {
	blocked  bool
	queries  []queries.PendingQuery
	recorded []string
}

func (f *exploreFixture) handler() *toolinteract.PageActions {
	guard := func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
		if f.blocked {
			return mcp.Fail(req, mcp.ErrCodePilotDisabled, "Pilot disabled", "enable pilot", opts...), true
		}
		return mcp.JSONRPCResponse{}, false
	}
	runtime := toolinteract.NewActionRuntime(toolinteract.RuntimeDeps{
		EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
			f.queries = append(f.queries, query)
			return mcp.JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, _ json.RawMessage, summary string) mcp.JSONRPCResponse {
			return mcp.Succeed(req, summary, map[string]any{"status": "queued", "correlation_id": correlationID})
		},
		RecordAIAction: func(action, _ string, _ map[string]any) { f.recorded = append(f.recorded, action) },
	})
	return toolinteract.NewPageActions(runtime, nil, nil, toolinteract.PageDeps{
		RequirePilot: guard, RequireExtension: allow, RequireTabTracking: allow,
	})
}

func allow(mcp.JSONRPCRequest, ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}

func request() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, ClientID: "explore-contract"}
}

func result(t *testing.T, response mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var got mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &got); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return got
}

func TestExplorePageRejectsMalformedAndUnsafeURLsBeforeDispatch(t *testing.T) {
	for _, args := range []string{
		`bad`,
		`{"url":"example.test"}`,
		`{"url":"javascript:alert(1)"}`,
		`{"url":"data:text/plain,no"}`,
		`{"url":"chrome://settings"}`,
		`{"url":"file:///tmp/private"}`,
		`{"url":"vbscript:msgbox(1)"}`,
		`{"url":"blob:https://example.test/id"}`,
	} {
		fixture := &exploreFixture{}
		got := result(t, fixture.handler().HandleExplorePage(request(), json.RawMessage(args)))
		if !got.IsError || len(fixture.queries) != 0 {
			t.Fatalf("args=%s result=%#v queries=%#v", args, got, fixture.queries)
		}
	}
}

func TestExplorePageForwardsCanonicalQueryAndTabTarget(t *testing.T) {
	for _, args := range []string{
		`{}`,
		`{"url":"http://example.test","visible_only":true,"limit":50,"tab_id":99}`,
		`{"url":"https://example.test"}`,
	} {
		fixture := &exploreFixture{}
		got := result(t, fixture.handler().HandleExplorePage(request(), json.RawMessage(args)))
		if got.IsError || len(fixture.queries) != 1 || fixture.queries[0].Type != "explore_page" ||
			!strings.HasPrefix(fixture.queries[0].CorrelationID, "explore_page_") || len(fixture.recorded) != 1 {
			t.Fatalf("args=%s result=%#v queries=%#v recorded=%#v", args, got, fixture.queries, fixture.recorded)
		}
		var params map[string]any
		if err := json.Unmarshal(fixture.queries[0].Params, &params); err != nil {
			t.Fatalf("decode query params: %v", err)
		}
		if strings.Contains(args, `"tab_id":99`) && fixture.queries[0].TabID != 99 {
			t.Fatalf("tab target = %d", fixture.queries[0].TabID)
		}
		if strings.Contains(args, `"visible_only":true`) && (params["visible_only"] != true || params["limit"] != float64(50)) {
			t.Fatalf("forwarded params = %#v", params)
		}
	}
}

func TestExplorePageStopsAtPilotGuard(t *testing.T) {
	fixture := &exploreFixture{blocked: true}
	got := result(t, fixture.handler().HandleExplorePage(request(), json.RawMessage(`{}`)))
	if !got.IsError || len(fixture.queries) != 0 || len(fixture.recorded) != 0 {
		t.Fatalf("result=%#v queries=%#v recorded=%#v", got, fixture.queries, fixture.recorded)
	}
}
