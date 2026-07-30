// toolobserve_coverage_test.go — Deterministic unit tests for observe-local handlers
// (inbox, push piggyback, page inventory, site menus) using in-memory fakes.

package toolobserve

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ---------------------------------------------------------------------------
// Fake Deps
// ---------------------------------------------------------------------------

type fakeDeps struct {
	inbox *push.PushInbox

	// EnqueuePendingQuery behavior.
	blockEnqueue bool
	blockResp    mcp.JSONRPCResponse
	lastQuery    queries.PendingQuery
	enqueued     bool

	// MaybeWaitForCommand behavior.
	waitResp        mcp.JSONRPCResponse
	waitCorrelation string
}

func (f *fakeDeps) EnqueuePendingQuery(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
	f.enqueued = true
	f.lastQuery = query
	if f.blockEnqueue {
		return f.blockResp, true
	}
	return mcp.JSONRPCResponse{}, false
}

func (f *fakeDeps) MaybeWaitForCommand(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
	f.waitCorrelation = correlationID
	return f.waitResp
}

func (f *fakeDeps) deps() Deps {
	return Deps{
		Inbox:               f.inbox,
		EnqueuePendingQuery: f.EnqueuePendingQuery,
		MaybeWaitForCommand: f.MaybeWaitForCommand,
	}
}

func testReq() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
}

func parseResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	return result
}

// ---------------------------------------------------------------------------
// HandleInbox
// ---------------------------------------------------------------------------

func TestHandleInbox_NilInbox(t *testing.T) {
	d := &fakeDeps{inbox: nil}
	resp := HandleInbox(d.deps(), testReq(), nil)
	result := parseResult(t, resp)
	if !strings.Contains(result.Content[0].Text, "Push inbox empty") {
		t.Errorf("expected empty inbox summary, got %q", result.Content[0].Text)
	}
}

func TestHandleInbox_EmptyInbox(t *testing.T) {
	d := &fakeDeps{inbox: push.NewPushInbox(10)}
	resp := HandleInbox(d.deps(), testReq(), nil)
	result := parseResult(t, resp)
	if !strings.Contains(result.Content[0].Text, "Push inbox empty") {
		t.Errorf("expected empty inbox summary, got %q", result.Content[0].Text)
	}
}

func TestHandleInbox_WithEvents(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{Type: "chat", Message: "hello", PageURL: "https://x"})
	inbox.Enqueue(push.PushEvent{Type: "chat", Message: "world", PageURL: "https://x"})
	d := &fakeDeps{inbox: inbox}

	resp := HandleInbox(d.deps(), testReq(), nil)
	result := parseResult(t, resp)
	if !strings.Contains(result.Content[0].Text, "Push inbox drained") {
		t.Errorf("expected drained summary, got %q", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, `"count":2`) {
		t.Errorf("expected count 2 in payload, got %q", result.Content[0].Text)
	}
	// Draining empties the inbox.
	if inbox.Len() != 0 {
		t.Errorf("inbox should be empty after drain, len=%d", inbox.Len())
	}
}

// ---------------------------------------------------------------------------
// AppendPushPiggyback
// ---------------------------------------------------------------------------

func baseResp(t *testing.T) mcp.JSONRPCResponse {
	t.Helper()
	return mcp.Succeed(testReq(), "base summary", map[string]any{"ok": true})
}

func TestAppendPushPiggyback_NilInbox(t *testing.T) {
	d := &fakeDeps{inbox: nil}
	resp := baseResp(t)
	got := AppendPushPiggyback(d.deps(), resp)
	if string(got.Result) != string(resp.Result) {
		t.Error("nil inbox should return response unchanged")
	}
}

func TestAppendPushPiggyback_EmptyInbox(t *testing.T) {
	d := &fakeDeps{inbox: push.NewPushInbox(10)}
	resp := baseResp(t)
	got := AppendPushPiggyback(d.deps(), resp)
	if string(got.Result) != string(resp.Result) {
		t.Error("empty inbox should return response unchanged")
	}
}

func TestAppendPushPiggyback_InvalidResultJSON(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{Type: "chat", Message: "hi", PageURL: "https://x"})
	d := &fakeDeps{inbox: inbox}

	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: json.RawMessage("not valid json")}
	got := AppendPushPiggyback(d.deps(), resp)
	if string(got.Result) != "not valid json" {
		t.Errorf("invalid result JSON should return response unchanged, got %s", string(got.Result))
	}
}

func TestAppendPushPiggyback_MixedEvents(t *testing.T) {
	inbox := push.NewPushInbox(20)
	inbox.Enqueue(push.PushEvent{
		Type: "annotations", PageURL: "https://a", AnnotSession: "sess-1",
		Annotations: json.RawMessage(`[{"note":"x"}]`),
	})
	inbox.Enqueue(push.PushEvent{Type: "chat", Message: "hey", PageURL: "https://b"})
	inbox.Enqueue(push.PushEvent{Type: "custom", PageURL: "https://c"})
	// Two distinct screenshots (different tab/url so no dedup).
	inbox.Enqueue(push.PushEvent{Type: "screenshot", TabID: 1, PageURL: "https://s1"})
	inbox.Enqueue(push.PushEvent{
		Type: "screenshot", TabID: 2, PageURL: "https://s2",
		Note: "final shot", ScreenshotB64: "QUJD",
	})
	d := &fakeDeps{inbox: inbox}

	got := AppendPushPiggyback(d.deps(), baseResp(t))
	result := parseResult(t, got)

	all := ""
	imageBlocks := 0
	for _, b := range result.Content {
		all += b.Text
		if b.Type == "image" {
			imageBlocks++
		}
	}
	if !strings.Contains(all, "_push_annotations") || !strings.Contains(all, "sess-1") {
		t.Errorf("expected annotation block with session, got %q", all)
	}
	if !strings.Contains(all, "_push_chat") || !strings.Contains(all, "hey") {
		t.Errorf("expected chat block, got %q", all)
	}
	if !strings.Contains(all, "_push_custom") {
		t.Errorf("expected default/custom block, got %q", all)
	}
	if !strings.Contains(all, "earlier screenshots skipped") {
		t.Errorf("expected skip summary for multiple screenshots, got %q", all)
	}
	if !strings.Contains(all, "final shot") {
		t.Errorf("expected screenshot note, got %q", all)
	}
	if imageBlocks != 1 {
		t.Errorf("expected exactly 1 image block, got %d", imageBlocks)
	}
}

func TestAppendPushPiggyback_SingleScreenshotNoSkip(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{Type: "screenshot", TabID: 1, PageURL: "https://only", ScreenshotB64: "QQ=="})
	d := &fakeDeps{inbox: inbox}

	got := AppendPushPiggyback(d.deps(), baseResp(t))
	result := parseResult(t, got)
	all := ""
	for _, b := range result.Content {
		all += b.Text
	}
	if strings.Contains(all, "earlier screenshots skipped") {
		t.Errorf("single screenshot should not emit skip summary, got %q", all)
	}
	if !strings.Contains(all, "_push_screenshot: captured from https://only") {
		t.Errorf("expected screenshot label, got %q", all)
	}
}

// ---------------------------------------------------------------------------
// HandlePageInventory
// ---------------------------------------------------------------------------

func TestHandlePageInventory_InvalidJSON(t *testing.T) {
	d := &fakeDeps{}
	resp := HandlePageInventory(d.deps(), testReq(), json.RawMessage(`{bad json`))
	result := parseResult(t, resp)
	if !result.IsError {
		t.Error("invalid JSON args should produce an error response")
	}
	if d.enqueued {
		t.Error("should not enqueue on invalid JSON")
	}
}

func TestHandlePageInventory_Blocked(t *testing.T) {
	sentinel := mcp.Succeed(testReq(), "blocked", map[string]any{"blocked": true})
	d := &fakeDeps{blockEnqueue: true, blockResp: sentinel}

	resp := HandlePageInventory(d.deps(), testReq(), json.RawMessage(`{"tab_id":5,"limit":10}`))
	if string(resp.Result) != string(sentinel.Result) {
		t.Error("blocked enqueue should return the block response")
	}
	if d.lastQuery.Type != "page_inventory" {
		t.Errorf("query type: want page_inventory, got %s", d.lastQuery.Type)
	}
	if d.lastQuery.TabID != 5 {
		t.Errorf("query TabID: want 5, got %d", d.lastQuery.TabID)
	}
}

func TestHandlePageInventory_QueuedSuccess(t *testing.T) {
	waitResp := mcp.Succeed(testReq(), "inventory ready", map[string]any{"elements": []any{}})
	d := &fakeDeps{waitResp: waitResp}

	resp := HandlePageInventory(d.deps(), testReq(), nil) // empty args path
	if string(resp.Result) != string(waitResp.Result) {
		t.Error("expected MaybeWaitForCommand response for queued inventory")
	}
	if d.waitCorrelation == "" || !strings.HasPrefix(d.waitCorrelation, "page_inventory_") {
		t.Errorf("correlation ID should be prefixed, got %q", d.waitCorrelation)
	}
}

// ---------------------------------------------------------------------------
// HandleSiteMenus
// ---------------------------------------------------------------------------

// listInteractiveResp builds a MaybeWaitForCommand-style response whose content
// is a summary line followed by a list_interactive JSON payload.
func listInteractiveResp(t *testing.T, elements []map[string]any) mcp.JSONRPCResponse {
	t.Helper()
	return mcp.Succeed(testReq(), "list interactive result", map[string]any{"elements": elements})
}

func TestHandleSiteMenus_Blocked(t *testing.T) {
	sentinel := mcp.Succeed(testReq(), "blocked", map[string]any{"blocked": true})
	d := &fakeDeps{blockEnqueue: true, blockResp: sentinel}

	resp := HandleSiteMenus(d.deps(), testReq(), nil)
	if string(resp.Result) != string(sentinel.Result) {
		t.Error("blocked enqueue should return the block response")
	}
	if d.lastQuery.Type != "dom_action" {
		t.Errorf("query type: want dom_action, got %s", d.lastQuery.Type)
	}
	var params map[string]any
	if err := json.Unmarshal(d.lastQuery.Params, &params); err != nil {
		t.Fatalf("query params: %v", err)
	}
	if params["action"] != "list_interactive" {
		t.Fatalf("DOM wire action = %v, want list_interactive", params["action"])
	}
	if _, obsolete := params["what"]; obsolete {
		t.Fatal("DOM wire params must not use the public MCP dispatch key")
	}
}

func TestHandleSiteMenus_ErrorResultPassthrough(t *testing.T) {
	errResp := mcp.Fail(testReq(), mcp.ErrInvalidJSON, "boom", "retry")
	d := &fakeDeps{waitResp: errResp}

	resp := HandleSiteMenus(d.deps(), testReq(), nil)
	result := parseResult(t, resp)
	if !result.IsError {
		t.Error("error result from wait should pass through unchanged")
	}
}

func TestHandleSiteMenus_NoElements(t *testing.T) {
	// Wait result with content that has no parseable JSON object -> empty menus.
	waitResp := mcp.SucceedText(testReq(), "no braces here just text")
	d := &fakeDeps{waitResp: waitResp}

	resp := HandleSiteMenus(d.deps(), testReq(), nil)
	result := parseResult(t, resp)
	if result.IsError {
		t.Error("no-elements case should still be a success response")
	}
	if !strings.Contains(result.Content[0].Text, "Site menus") {
		t.Errorf("expected 'Site menus' summary, got %q", result.Content[0].Text)
	}
}

func TestHandleSiteMenus_WithElementsFull(t *testing.T) {
	elements := []map[string]any{
		{"index": 0, "tag": "a", "label": "Home", "href": "/home", "visible": true, "role": "link"},
		{"index": 1, "tag": "a", "label": "About", "href": "/about", "visible": true, "role": "link"},
	}
	d := &fakeDeps{waitResp: listInteractiveResp(t, elements)}

	resp := HandleSiteMenus(d.deps(), testReq(), json.RawMessage(`{"summary":false}`))
	result := parseResult(t, resp)
	if result.IsError {
		t.Errorf("expected success, got error: %q", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Site menus") {
		t.Errorf("expected 'Site menus' summary line, got %q", result.Content[0].Text)
	}
}

func TestHandleSiteMenus_WithElementsSummary(t *testing.T) {
	elements := []map[string]any{
		{"index": 0, "tag": "a", "label": "Home", "href": "/home", "visible": true, "role": "link"},
	}
	d := &fakeDeps{waitResp: listInteractiveResp(t, elements)}

	resp := HandleSiteMenus(d.deps(), testReq(), json.RawMessage(`{"summary":true}`))
	result := parseResult(t, resp)
	if result.IsError {
		t.Errorf("expected success summary, got error: %q", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Site menus summary") {
		t.Errorf("expected summary variant, got %q", result.Content[0].Text)
	}
	// Summary payload should include count keys.
	if !strings.Contains(result.Content[0].Text, "ungrouped_count") {
		t.Errorf("expected summary counts, got %q", result.Content[0].Text)
	}
}

// ---------------------------------------------------------------------------
// parseListInteractiveResult direct edge cases
// ---------------------------------------------------------------------------

func TestParseListInteractiveResult_EmptyContent(t *testing.T) {
	if got := parseListInteractiveResult(mcp.MCPToolResult{}); got != nil {
		t.Errorf("empty content should return nil, got %v", got)
	}
}

func TestParseListInteractiveResult_NoJSONObject(t *testing.T) {
	result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: "no json"}}}
	if got := parseListInteractiveResult(result); got != nil {
		t.Errorf("text without JSON should return nil, got %v", got)
	}
}

func TestParseListInteractiveResult_InvalidJSON(t *testing.T) {
	result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: "summary\n{not valid"}}}
	if got := parseListInteractiveResult(result); got != nil {
		t.Errorf("invalid JSON should return nil, got %v", got)
	}
}

func TestParseListInteractiveResult_Valid(t *testing.T) {
	text := `elements found` + "\n" + `{"elements":[{"index":0,"tag":"a","label":"Home","href":"/h","visible":true}]}`
	result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: text}}}
	got := parseListInteractiveResult(result)
	if len(got) != 1 {
		t.Fatalf("expected 1 element, got %d", len(got))
	}
	if got[0].Text != "Home" || got[0].Href != "/h" || got[0].Tag != "a" {
		t.Errorf("unexpected parsed element: %+v", got[0])
	}
}

// ---------------------------------------------------------------------------
// helpers: newCorrelationID
// ---------------------------------------------------------------------------

func TestNewCorrelationID_UniqueAndPrefixed(t *testing.T) {
	a := toolresp.NewCorrelationID("obs")
	b := toolresp.NewCorrelationID("obs")
	if !strings.HasPrefix(a, "obs_") {
		t.Errorf("expected prefix, got %q", a)
	}
	if a == b {
		t.Error("correlation IDs should be unique")
	}
}
