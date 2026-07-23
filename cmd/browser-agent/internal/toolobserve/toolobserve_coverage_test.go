// toolobserve_coverage_test.go — Behavior tests for observe-local handlers:
// correlation IDs, page_inventory / site_menus dispatch contracts, and push piggyback.

package toolobserve

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// recorderDeps records every dispatch so tests can assert the exact query
// shape handed to the extension, and lets tests script the responses.
type recorderDeps struct {
	inbox     *push.PushInbox
	connected bool

	// Recorded calls.
	queries         []queries.PendingQuery
	enqueueTimeouts []time.Duration
	waitCorrIDs     []string
	waitArgs        []json.RawMessage
	waitSummaries   []string

	// Scripted results.
	blockWith *mcp.JSONRPCResponse // non-nil => EnqueuePendingQuery reports blocked
	waitWith  mcp.JSONRPCResponse
}

func (d *recorderDeps) PushInbox() *push.PushInbox { return d.inbox }
func (d *recorderDeps) IsExtensionConnected() bool { return d.connected }

func (d *recorderDeps) EnqueuePendingQuery(_ mcp.JSONRPCRequest, q queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
	d.queries = append(d.queries, q)
	d.enqueueTimeouts = append(d.enqueueTimeouts, timeout)
	if d.blockWith != nil {
		return *d.blockWith, true
	}
	return mcp.JSONRPCResponse{}, false
}

func (d *recorderDeps) MaybeWaitForCommand(_ mcp.JSONRPCRequest, correlationID string, args json.RawMessage, summary string) mcp.JSONRPCResponse {
	d.waitCorrIDs = append(d.waitCorrIDs, correlationID)
	d.waitArgs = append(d.waitArgs, args)
	d.waitSummaries = append(d.waitSummaries, summary)
	return d.waitWith
}

// toolResult unmarshals a JSONRPCResponse's MCP tool result.
func toolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var r mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	return r
}

// payload extracts the JSON object appended after the summary line.
func payload(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	r := toolResult(t, resp)
	if len(r.Content) == 0 {
		t.Fatal("response has no content blocks")
	}
	text := r.Content[0].Text
	i := strings.IndexByte(text, '{')
	if i < 0 {
		t.Fatalf("no JSON payload in %q", text)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text[i:]), &m); err != nil {
		t.Fatalf("unmarshal payload %q: %v", text[i:], err)
	}
	return m
}

// errorCode returns the structured error_code from an isError response.
func errorCode(t *testing.T, resp mcp.JSONRPCResponse) string {
	t.Helper()
	r := toolResult(t, resp)
	if !r.IsError {
		t.Fatalf("expected an error response, got %s", string(resp.Result))
	}
	m := payload(t, resp)
	code, _ := m["error_code"].(string)
	return code
}

// mcpTextResult builds a JSONRPCResponse whose tool result carries the given
// summary + JSON body, mimicking what MaybeWaitForCommand returns.
func mcpTextResult(text string, isError bool) mcp.JSONRPCResponse {
	raw, _ := json.Marshal(mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: text}},
		IsError: isError,
	})
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: json.RawMessage(`1`), Result: raw}
}

// ---------------------------------------------------------------------------
// newCorrelationID
// ---------------------------------------------------------------------------

func TestNewCorrelationID_KeepsPrefixAndIsUnique(t *testing.T) {
	t.Parallel()

	const n = 500
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := newCorrelationID("page_inventory")
		if !strings.HasPrefix(id, "page_inventory_") {
			t.Fatalf("id %q lost its prefix — observe(what:\"command_result\") routing depends on it", id)
		}
		// prefix + "_" + unix nanos + "_" + random int64
		rest := strings.TrimPrefix(id, "page_inventory_")
		parts := strings.Split(rest, "_")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			t.Fatalf("id %q does not have the <prefix>_<nanos>_<rand> shape", id)
		}
		if seen[id] {
			t.Fatalf("duplicate correlation id %q after %d calls", id, i)
		}
		seen[id] = true
	}
}

func TestRandomInt63_ProducesVaryingValues(t *testing.T) {
	t.Parallel()

	// A constant here would collapse correlation IDs generated within the same
	// nanosecond onto each other.
	first := randomInt63()
	distinct := false
	for i := 0; i < 100; i++ {
		if randomInt63() != first {
			distinct = true
			break
		}
	}
	if !distinct {
		t.Error("randomInt63 returned the same value 101 times")
	}
}

// ---------------------------------------------------------------------------
// HandlePageInventory
// ---------------------------------------------------------------------------

func TestHandlePageInventory_DispatchesPageInventoryQueryWithTabID(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{waitWith: mcpTextResult("Page inventory queued\n{}", false)}
	args := json.RawMessage(`{"tab_id":42,"visible_only":true,"limit":25}`)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: json.RawMessage(`7`)}

	HandlePageInventory(d, req, args)

	if len(d.queries) != 1 {
		t.Fatalf("enqueued %d queries, want 1", len(d.queries))
	}
	q := d.queries[0]
	if q.Type != "page_inventory" {
		t.Errorf("query type = %q, want page_inventory", q.Type)
	}
	if q.TabID != 42 {
		t.Errorf("query tab_id = %d, want 42 (tab targeting must survive parsing)", q.TabID)
	}
	if string(q.Params) != string(args) {
		t.Errorf("query params = %s, want the caller args verbatim %s", q.Params, args)
	}
	if !strings.HasPrefix(q.CorrelationID, "page_inventory_") {
		t.Errorf("correlation id = %q, want page_inventory_ prefix", q.CorrelationID)
	}
	if d.enqueueTimeouts[0] != queries.AsyncCommandTimeout {
		t.Errorf("enqueue timeout = %v, want queries.AsyncCommandTimeout", d.enqueueTimeouts[0])
	}
	// The same correlation ID must be handed to the waiter, or the result is orphaned.
	if len(d.waitCorrIDs) != 1 || d.waitCorrIDs[0] != q.CorrelationID {
		t.Errorf("wait correlation ids = %v, want [%s]", d.waitCorrIDs, q.CorrelationID)
	}
	if d.waitSummaries[0] != "Page inventory queued" {
		t.Errorf("queued summary = %q, want %q", d.waitSummaries[0], "Page inventory queued")
	}
}

func TestHandlePageInventory_ReturnsWaiterResponse(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{waitWith: mcpTextResult(`Page inventory\n{"elements":[]}`, false)}
	resp := HandlePageInventory(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	if string(resp.Result) != string(d.waitWith.Result) {
		t.Errorf("handler must return the waiter response unchanged; got %s", resp.Result)
	}
}

func TestHandlePageInventory_NilArgsStillDispatches(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{waitWith: mcpTextResult("ok\n{}", false)}
	resp := HandlePageInventory(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	if toolResult(t, resp).IsError {
		t.Fatal("nil args must not be an error — every field is optional")
	}
	if len(d.queries) != 1 || d.queries[0].TabID != 0 {
		t.Errorf("want one query with tab_id 0, got %+v", d.queries)
	}
}

func TestHandlePageInventory_MalformedJSONRejectedBeforeDispatch(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{}
	resp := HandlePageInventory(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"tab_id":`))

	if got := errorCode(t, resp); got != mcp.ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", got, mcp.ErrInvalidJSON)
	}
	if len(d.queries) != 0 {
		t.Error("malformed args must not reach the extension queue")
	}
}

func TestHandlePageInventory_WrongTypeForTabIDRejected(t *testing.T) {
	t.Parallel()

	// Strict unmarshal: a string tab_id is a caller bug, not a default.
	d := &recorderDeps{}
	resp := HandlePageInventory(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"tab_id":"42"}`))

	if got := errorCode(t, resp); got != mcp.ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", got, mcp.ErrInvalidJSON)
	}
	if len(d.queries) != 0 {
		t.Error("type-mismatched args must not reach the extension queue")
	}
}

func TestHandlePageInventory_BlockedEnqueueShortCircuits(t *testing.T) {
	t.Parallel()

	blocked := mcpTextResult("queue full", true)
	d := &recorderDeps{blockWith: &blocked}
	resp := HandlePageInventory(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	if string(resp.Result) != string(blocked.Result) {
		t.Errorf("blocked enqueue response must be returned verbatim; got %s", resp.Result)
	}
	if len(d.waitCorrIDs) != 0 {
		t.Error("must not wait for a command that was never queued")
	}
}

// ---------------------------------------------------------------------------
// HandleSiteMenus
// ---------------------------------------------------------------------------

// listInteractiveJSON is a realistic extension list_interactive payload: a 3-link
// <nav>, a 2-link <footer>, and one unattached button.
const listInteractiveJSON = `Interactive elements
{"elements":[
 {"index":0,"tag":"a","label":"Home","href":"/","visible":true,"landmark_tag":"nav","bbox":{"x":10,"y":10,"width":60,"height":20}},
 {"index":1,"tag":"a","label":"Docs","href":"/docs","visible":true,"landmark_tag":"nav","bbox":{"x":80,"y":10,"width":60,"height":20}},
 {"index":2,"tag":"a","label":"Pricing","href":"/pricing","visible":true,"landmark_tag":"nav","bbox":{"x":150,"y":10,"width":60,"height":20}},
 {"index":3,"tag":"a","label":"Privacy","href":"/privacy","visible":true,"landmark_tag":"footer","bbox":{"x":10,"y":860,"width":60,"height":20}},
 {"index":4,"tag":"a","label":"Terms","href":"/terms","visible":true,"landmark_tag":"footer","bbox":{"x":80,"y":860,"width":60,"height":20}},
 {"index":5,"tag":"button","label":"Search","visible":true,"bbox":{"x":600,"y":400,"width":40,"height":20}}
]}`

func TestHandleSiteMenus_DispatchesVisibleOnlyListInteractive(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{waitWith: mcpTextResult(listInteractiveJSON, false)}
	args := json.RawMessage(`{"summary":false}`)
	HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, args)

	if len(d.queries) != 1 {
		t.Fatalf("enqueued %d queries, want 1", len(d.queries))
	}
	q := d.queries[0]
	// site_menus is a Go-side heuristic layered on a plain dom_action dispatch.
	if q.Type != "dom_action" {
		t.Errorf("query type = %q, want dom_action", q.Type)
	}
	var sent map[string]any
	if err := json.Unmarshal(q.Params, &sent); err != nil {
		t.Fatalf("query params are not JSON: %v", err)
	}
	if sent["what"] != "list_interactive" {
		t.Errorf("params[what] = %v, want list_interactive", sent["what"])
	}
	if sent["visible_only"] != true {
		t.Errorf("params[visible_only] = %v, want true — off-screen controls are not menus", sent["visible_only"])
	}
	if !strings.HasPrefix(q.CorrelationID, "site_menus_") {
		t.Errorf("correlation id = %q, want site_menus_ prefix", q.CorrelationID)
	}
	if d.waitSummaries[0] != "site_menus queued" {
		t.Errorf("queued summary = %q, want %q", d.waitSummaries[0], "site_menus queued")
	}
	if string(d.waitArgs[0]) != string(args) {
		t.Errorf("waiter args = %s, want the caller args %s", d.waitArgs[0], args)
	}
}

func TestHandleSiteMenus_GroupsLandmarkElementsByBucket(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{waitWith: mcpTextResult(listInteractiveJSON, false)}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	m := payload(t, resp)

	main, _ := m["main"].([]any)
	if len(main) != 1 {
		t.Fatalf("main groups = %d, want 1 (the <nav> landmark)", len(main))
	}
	group, _ := main[0].(map[string]any)
	if group["classification"] != "main" {
		t.Errorf("classification = %v, want main", group["classification"])
	}
	if group["source"] != "semantic" {
		t.Errorf("source = %v, want semantic (landmark_tag wins over geometry)", group["source"])
	}
	items, _ := group["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("nav items = %d, want 3", len(items))
	}
	first, _ := items[0].(map[string]any)
	// label -> text and href are the fields an agent navigates with.
	if first["text"] != "Home" || first["href"] != "/" {
		t.Errorf("first nav item = %v, want text=Home href=/", first)
	}
	if _, leaked := first["bbox"]; leaked {
		t.Error("bbox must be stripped from menu items in the API payload")
	}

	footer, _ := m["footer"].([]any)
	if len(footer) != 1 {
		t.Fatalf("footer groups = %d, want 1", len(footer))
	}
	fitems, _ := footer[0].(map[string]any)["items"].([]any)
	if len(fitems) != 2 {
		t.Errorf("footer items = %d, want 2", len(fitems))
	}

	ungrouped, _ := m["ungrouped"].([]any)
	if len(ungrouped) != 1 {
		t.Fatalf("ungrouped = %d, want 1 (the lone Search button)", len(ungrouped))
	}
	if ungrouped[0].(map[string]any)["text"] != "Search" {
		t.Errorf("ungrouped item = %v, want text=Search", ungrouped[0])
	}
}

func TestHandleSiteMenus_SummaryReturnsCountsOnly(t *testing.T) {
	t.Parallel()

	d := &recorderDeps{waitWith: mcpTextResult(listInteractiveJSON, false)}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"summary":true}`))

	m := payload(t, resp)
	want := map[string]float64{
		"main_count": 3, "footer_count": 2, "sidebar_count": 0, "other_count": 0,
		"ungrouped_count": 1, "main_groups": 1, "footer_groups": 1,
		"sidebar_groups": 0, "other_groups": 0,
	}
	for key, wantVal := range want {
		got, ok := m[key].(float64)
		if !ok {
			t.Errorf("summary missing %q (payload=%v)", key, m)
			continue
		}
		if got != wantVal {
			t.Errorf("summary[%q] = %v, want %v", key, got, wantVal)
		}
	}
	if _, ok := m["main"]; ok {
		t.Error("summary must not include the full menu tree")
	}
}

func TestHandleSiteMenus_InvisibleElementsAreNotMenus(t *testing.T) {
	t.Parallel()

	body := `Interactive
{"elements":[
 {"index":0,"tag":"a","label":"Hidden","visible":false,"landmark_tag":"nav","bbox":{"x":0,"y":0,"width":10,"height":10}}
]}`
	d := &recorderDeps{waitWith: mcpTextResult(body, false)}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	m := payload(t, resp)
	if main, _ := m["main"].([]any); len(main) != 0 {
		t.Errorf("main = %v, want empty — a hidden nav link is not a menu item", main)
	}
	if ung, _ := m["ungrouped"].([]any); len(ung) != 0 {
		t.Errorf("ungrouped = %v, want empty", ung)
	}
}

func TestHandleSiteMenus_UnparseableResultReturnsEmptyArraysNotNull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"no json object in text", "Interactive elements: none"},
		{"malformed json body", "Interactive\n{\"elements\": [ oops"},
		{"elements is the wrong type", "Interactive\n{\"elements\":\"none\"}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &recorderDeps{waitWith: mcpTextResult(tt.body, false)}
			resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

			if toolResult(t, resp).IsError {
				t.Fatal("an unparseable extension payload should degrade to empty menus, not an error")
			}
			m := payload(t, resp)
			// Clients index these directly; null would be a runtime error for them.
			for _, key := range []string{"main", "sidebar", "footer", "other", "ungrouped"} {
				v, ok := m[key]
				if !ok {
					t.Errorf("payload missing %q", key)
					continue
				}
				arr, isArr := v.([]any)
				if !isArr {
					t.Errorf("%s = %v (%T), want an empty array", key, v, v)
					continue
				}
				if len(arr) != 0 {
					t.Errorf("%s = %v, want empty", key, arr)
				}
			}
		})
	}
}

func TestHandleSiteMenus_EmptyContentReturnsEmptyMenus(t *testing.T) {
	t.Parallel()

	raw, _ := json.Marshal(mcp.MCPToolResult{Content: nil})
	d := &recorderDeps{waitWith: mcp.JSONRPCResponse{ID: json.RawMessage(`1`), Result: raw}}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	m := payload(t, resp)
	if main, _ := m["main"].([]any); len(main) != 0 {
		t.Errorf("main = %v, want empty", main)
	}
}

func TestHandleSiteMenus_ExtensionErrorIsReturnedUnchanged(t *testing.T) {
	t.Parallel()

	extErr := mcpTextResult("Error: extension_timeout — retry\n{\"error_code\":\"extension_timeout\"}", true)
	d := &recorderDeps{waitWith: extErr}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	// Menu heuristics must not paper over a failed dispatch with an empty result.
	if string(resp.Result) != string(extErr.Result) {
		t.Errorf("resp = %s, want the extension error verbatim", resp.Result)
	}
}

func TestHandleSiteMenus_BlockedEnqueueShortCircuits(t *testing.T) {
	t.Parallel()

	blocked := mcpTextResult("Error: queue_full — retry later\n{\"error_code\":\"queue_full\"}", true)
	d := &recorderDeps{blockWith: &blocked}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)

	if string(resp.Result) != string(blocked.Result) {
		t.Errorf("resp = %s, want the blocked-enqueue response verbatim", resp.Result)
	}
	if len(d.waitCorrIDs) != 0 {
		t.Error("must not wait for a command that was never queued")
	}
}

func TestHandleSiteMenus_MalformedSummaryFlagIsIgnored(t *testing.T) {
	t.Parallel()

	// site_menus parses params leniently: a bad optional flag falls back to the
	// full (non-summary) response instead of failing the call.
	d := &recorderDeps{waitWith: mcpTextResult(listInteractiveJSON, false)}
	resp := HandleSiteMenus(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"summary":"yes"}`))

	m := payload(t, resp)
	if _, ok := m["main"]; !ok {
		t.Errorf("want the full menu tree, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// AppendPushPiggyback
// ---------------------------------------------------------------------------

func baseResponse() mcp.JSONRPCResponse {
	return mcpTextResult("Logs\n{\"count\":0}", false)
}

func TestAppendPushPiggyback_NoEventsLeavesResponseByteIdentical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		inbox *push.PushInbox
	}{
		{"nil inbox", nil},
		{"empty inbox", push.NewPushInbox(4)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := baseResponse()
			got := AppendPushPiggyback(&recorderDeps{inbox: tt.inbox}, orig)
			if string(got.Result) != string(orig.Result) {
				t.Errorf("result changed with no events:\n got %s\nwant %s", got.Result, orig.Result)
			}
		})
	}
}

func TestAppendPushPiggyback_UnparseableResponseIsLeftAloneAndEventsAreConsumed(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{ID: "a", Type: "chat", Message: "hi"})

	orig := mcp.JSONRPCResponse{ID: json.RawMessage(`1`), Result: json.RawMessage(`not json`)}
	got := AppendPushPiggyback(&recorderDeps{inbox: inbox}, orig)

	if string(got.Result) != "not json" {
		t.Errorf("result = %s, want it untouched", got.Result)
	}
	// Documents current behavior: DrainAll runs before the unmarshal check, so
	// the events are dropped rather than left for the next call.
	if inbox.Len() != 0 {
		t.Errorf("inbox len = %d, want 0 (events were already drained)", inbox.Len())
	}
}

func TestAppendPushPiggyback_ChatEventBecomesTextBlock(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{ID: "a", Type: "chat", Message: "please retry", PageURL: "https://app.test/x"})

	got := AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse())
	r := toolResult(t, got)

	if len(r.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2 (original + chat)", len(r.Content))
	}
	block := r.Content[1]
	if block.Type != "text" {
		t.Errorf("block type = %q, want text", block.Type)
	}
	// The _push_ marker is how the agent tells piggybacked events from tool output.
	if !strings.Contains(block.Text, "_push_chat: please retry") {
		t.Errorf("block = %q, want the _push_chat marker and message", block.Text)
	}
	if !strings.Contains(block.Text, "[from: https://app.test/x]") {
		t.Errorf("block = %q, want the originating page URL", block.Text)
	}
}

func TestAppendPushPiggyback_AnnotationEventCarriesSessionAndPayload(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{
		ID: "a", Type: "annotations", PageURL: "https://app.test/",
		AnnotSession: "review-1",
		Annotations:  json.RawMessage(`[{"note":"too small"}]`),
	})

	got := AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse())
	r := toolResult(t, got)
	if len(r.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(r.Content))
	}
	text := r.Content[1].Text
	for _, want := range []string{"_push_annotations: from https://app.test/", "(session: review-1)", `[{"note":"too small"}]`} {
		if !strings.Contains(text, want) {
			t.Errorf("annotation block %q missing %q", text, want)
		}
	}
}

func TestAppendPushPiggyback_AnnotationEventWithoutSessionOrPayload(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{ID: "a", Type: "annotations", PageURL: "https://app.test/"})

	r := toolResult(t, AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse()))
	text := r.Content[1].Text
	if strings.Contains(text, "session:") {
		t.Errorf("block %q should omit the session suffix when none is set", text)
	}
	if strings.TrimSpace(text) != "_push_annotations: from https://app.test/" {
		t.Errorf("block = %q, want just the bare annotation label", text)
	}
}

func TestAppendPushPiggyback_UnknownEventTypeFallsBackToGenericLabel(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{ID: "a", Type: "console_error", PageURL: "https://app.test/"})

	r := toolResult(t, AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse()))
	if len(r.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2 — unknown types must still surface", len(r.Content))
	}
	if !strings.Contains(r.Content[1].Text, "_push_console_error: event from https://app.test/") {
		t.Errorf("block = %q, want the generic _push_<type> label", r.Content[1].Text)
	}
}

func TestAppendPushPiggyback_SingleScreenshotAddsLabelAndImageBlock(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{
		ID: "s1", Type: "screenshot", PageURL: "https://app.test/a",
		Note: "before submit", ScreenshotB64: "QUJD",
	})

	r := toolResult(t, AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse()))
	if len(r.Content) != 3 {
		t.Fatalf("content blocks = %d, want 3 (original + label + image)", len(r.Content))
	}
	if !strings.Contains(r.Content[1].Text, "_push_screenshot: captured from https://app.test/a — before submit") {
		t.Errorf("label = %q, want URL and note", r.Content[1].Text)
	}
	img := r.Content[2]
	if img.Type != "image" {
		t.Errorf("block type = %q, want image", img.Type)
	}
	if img.Data != "QUJD" {
		t.Errorf("image data = %q, want the base64 payload", img.Data)
	}
	// MCP image blocks use camelCase mimeType per the protocol spec.
	if img.MimeType != "image/jpeg" {
		t.Errorf("mimeType = %q, want image/jpeg", img.MimeType)
	}
}

func TestAppendPushPiggyback_ScreenshotWithoutImageDataOmitsImageBlock(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{ID: "s1", Type: "screenshot", PageURL: "https://app.test/a"})

	r := toolResult(t, AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse()))
	if len(r.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2 (original + label, no empty image)", len(r.Content))
	}
	if r.Content[1].Text != "\n\n_push_screenshot: captured from https://app.test/a" {
		t.Errorf("label = %q, want no trailing note separator", r.Content[1].Text)
	}
}

func TestAppendPushPiggyback_OnlyMostRecentScreenshotIsDelivered(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(8)
	// Distinct page URLs so the inbox's consecutive-screenshot dedup does not fire.
	inbox.Enqueue(push.PushEvent{ID: "s1", Type: "screenshot", PageURL: "https://app.test/1", ScreenshotB64: "AAA"})
	inbox.Enqueue(push.PushEvent{ID: "s2", Type: "screenshot", PageURL: "https://app.test/2", ScreenshotB64: "BBB"})
	inbox.Enqueue(push.PushEvent{ID: "s3", Type: "screenshot", PageURL: "https://app.test/3", ScreenshotB64: "CCC"})

	r := toolResult(t, AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse()))

	images := 0
	var imgData string
	for _, b := range r.Content {
		if b.Type == "image" {
			images++
			imgData = b.Data
		}
	}
	if images != 1 {
		t.Fatalf("image blocks = %d, want exactly 1 — piggybacking every screenshot blows the context window", images)
	}
	if imgData != "CCC" {
		t.Errorf("delivered image = %q, want the most recent (CCC)", imgData)
	}
	joined := ""
	for _, b := range r.Content {
		joined += b.Text
	}
	if !strings.Contains(joined, "_push_screenshot: 2 earlier screenshots skipped (showing most recent only)") {
		t.Errorf("missing skip summary; blocks = %q", joined)
	}
}

func TestAppendPushPiggyback_NonScreenshotEventsComeBeforeScreenshots(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(8)
	inbox.Enqueue(push.PushEvent{ID: "s1", Type: "screenshot", PageURL: "https://app.test/1", ScreenshotB64: "AAA"})
	inbox.Enqueue(push.PushEvent{ID: "c1", Type: "chat", Message: "look here", PageURL: "https://app.test/1"})

	r := toolResult(t, AppendPushPiggyback(&recorderDeps{inbox: inbox}, baseResponse()))
	if len(r.Content) != 4 {
		t.Fatalf("content blocks = %d, want 4 (original + chat + screenshot label + image)", len(r.Content))
	}
	// The image is last so the model reads the text context before the picture,
	// regardless of the order events arrived in.
	if !strings.Contains(r.Content[1].Text, "_push_chat") {
		t.Errorf("block 1 = %q, want the chat event first", r.Content[1].Text)
	}
	if r.Content[3].Type != "image" {
		t.Errorf("block 3 type = %q, want image last", r.Content[3].Type)
	}
}

func TestAppendPushPiggyback_DrainsInbox(t *testing.T) {
	t.Parallel()

	inbox := push.NewPushInbox(4)
	inbox.Enqueue(push.PushEvent{ID: "a", Type: "chat", Message: "one"})
	d := &recorderDeps{inbox: inbox}

	AppendPushPiggyback(d, baseResponse())
	if inbox.Len() != 0 {
		t.Fatalf("inbox len = %d, want 0", inbox.Len())
	}

	// A second call must not repeat the same event.
	again := AppendPushPiggyback(d, baseResponse())
	if len(toolResult(t, again).Content) != 1 {
		t.Error("second call re-delivered already-consumed events")
	}
}
