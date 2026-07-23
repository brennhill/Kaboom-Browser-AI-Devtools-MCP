// interact_dom_coverage_test.go — Behavioural tests for the DOM primitive, browser
// action, upload, clipboard, draw, explore and content-extraction handlers.
// Covers: interact_dom*.go, interact_upload.go, interact_browser_*.go,
// interact_clipboard.go, interact_draw.go, interact_explore.go, interact_content.go.
// All package-scope identifiers are prefixed `domcov`.

package toolinteract

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// domcovQuery records one command that reached EnqueuePendingQuery.
type domcovQuery struct {
	Type          string
	CorrelationID string
	TabID         int
	Timeout       time.Duration
	Params        map[string]any
	RawParams     json.RawMessage
}

// domcovAIAction records one RecordAIAction call.
type domcovAIAction struct {
	Action string
	URL    string
	Extra  map[string]any
}

// domcovPrimitive records one RecordDOMPrimitiveAction call.
type domcovPrimitive struct {
	Action, Selector, Text, Value string
}

// domcovHarness wires a fully stubbed Deps so handlers run end to end with no
// browser, no network and no main package.
type domcovHarness struct {
	t   *testing.T
	h   *InteractActionHandler
	up  *UploadInteractHandler
	cap *capture.Store

	// Recorded effects.
	queries    []domcovQuery
	guardCalls []string
	guardOpts  []func(*StructuredError)
	aiActions  []domcovAIAction
	primitives []domcovPrimitive
	cspWorlds  []string
	waitCalls  []string // "<correlationID>|<queuedMessage>"

	captureCalls    int
	drawStarted     int
	screenshotCalls int
	pageInfoCalls   int
	enrichCalls     int
	enrichTabIDs    []int
	injectCalls     int

	// Knobs — set before invoking a handler.
	blockPilot     bool
	blockExt       bool
	blockTab       bool
	blockCSP       bool
	blockEnqueue   bool
	listenPort     int
	screenshot     func(req JSONRPCRequest) JSONRPCResponse
	waitResponse   func(req JSONRPCRequest, correlationID string, queued string) JSONRPCResponse
	enrichNavigate func(resp JSONRPCResponse) JSONRPCResponse
}

func domcovNew(t *testing.T) *domcovHarness {
	t.Helper()
	hs := &domcovHarness{t: t, cap: capture.NewCapture(), listenPort: 7890}

	guard := func(name string, blocked *bool) GuardCheck {
		return func(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool) {
			hs.guardCalls = append(hs.guardCalls, name)
			if len(opts) > 0 {
				hs.guardOpts = opts
			}
			if *blocked {
				return fail(req, ErrCodePilotDisabled, name+" is blocked", "Enable "+name, opts...), true
			}
			return JSONRPCResponse{}, false
		}
	}

	deps := &Deps{
		RequirePilot:       guard("pilot", &hs.blockPilot),
		RequireExtension:   guard("extension", &hs.blockExt),
		RequireTabTracking: guard("tab_tracking", &hs.blockTab),
		RequireCSPClear: func(req JSONRPCRequest, world string) (JSONRPCResponse, bool) {
			hs.cspWorlds = append(hs.cspWorlds, world)
			if hs.blockCSP {
				return fail(req, ErrInvalidParam, "csp blocked world="+world, "Use isolated"), true
			}
			return JSONRPCResponse{}, false
		},
		Capture: func() *capture.Store {
			hs.captureCalls++
			return hs.cap
		},
		EnqueuePendingQuery: func(req JSONRPCRequest, q queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool) {
			rec := domcovQuery{
				Type:          q.Type,
				CorrelationID: q.CorrelationID,
				TabID:         q.TabID,
				Timeout:       timeout,
				Params:        map[string]any{},
				RawParams:     q.Params,
			}
			_ = json.Unmarshal(q.Params, &rec.Params)
			hs.queries = append(hs.queries, rec)
			if hs.blockEnqueue {
				return fail(req, ErrQueueFull, "queue is full", "Retry later"), true
			}
			return JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req JSONRPCRequest, correlationID string, _ json.RawMessage, queued string) JSONRPCResponse {
			hs.waitCalls = append(hs.waitCalls, correlationID+"|"+queued)
			if hs.waitResponse != nil {
				return hs.waitResponse(req, correlationID, queued)
			}
			return succeed(req, queued, map[string]any{
				"status":         "queued",
				"correlation_id": correlationID,
			})
		},
		RecordAIAction: func(action, url string, extra map[string]any) {
			hs.aiActions = append(hs.aiActions, domcovAIAction{Action: action, URL: url, Extra: extra})
		},
		RecordDOMPrimitiveAction: func(action, selector, text, value string) {
			hs.primitives = append(hs.primitives, domcovPrimitive{action, selector, text, value})
		},
		MarkDrawStarted: func() { hs.drawStarted++ },
		GetScreenshot: func(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
			hs.screenshotCalls++
			if hs.screenshot != nil {
				return hs.screenshot(req)
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID}
		},
		GetPageInfo: func(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
			hs.pageInfoCalls++
			return fail(req, ErrNoData, "no page info", "n/a")
		},
		EnrichNavigateResponse: func(resp JSONRPCResponse, _ JSONRPCRequest, tabID int) JSONRPCResponse {
			hs.enrichCalls++
			hs.enrichTabIDs = append(hs.enrichTabIDs, tabID)
			if hs.enrichNavigate != nil {
				return hs.enrichNavigate(resp)
			}
			return resp
		},
		InjectCSPBlockedActions: func(resp JSONRPCResponse) JSONRPCResponse {
			hs.injectCalls++
			return resp
		},
		GetListenPort: func() int { return hs.listenPort },
	}
	hs.h = NewInteractActionHandler(deps)
	hs.up = NewUploadInteractHandler(deps, hs.h)
	return hs
}

// domcovOnlyQuery returns the single enqueued query, failing if there is not exactly one.
func (hs *domcovHarness) domcovOnlyQuery() domcovQuery {
	hs.t.Helper()
	if len(hs.queries) != 1 {
		hs.t.Fatalf("expected exactly 1 enqueued query, got %d: %+v", len(hs.queries), hs.queries)
	}
	return hs.queries[0]
}

// domcovOnlyAI returns the single recorded AI action.
func (hs *domcovHarness) domcovOnlyAI() domcovAIAction {
	hs.t.Helper()
	if len(hs.aiActions) != 1 {
		hs.t.Fatalf("expected exactly 1 recorded AI action, got %d: %+v", len(hs.aiActions), hs.aiActions)
	}
	return hs.aiActions[0]
}

// ---------------------------------------------------------------------------
// Shared readers
// ---------------------------------------------------------------------------

func domcovReq() JSONRPCRequest {
	return JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: float64(1), ClientID: "domcov-client"}
}

// domcovError asserts the response is an error and returns its StructuredError.
func domcovError(t *testing.T, resp JSONRPCResponse) StructuredError {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	if !result.IsError {
		t.Fatalf("expected an error response, got success: %s", firstText(result))
	}
	text := firstText(result)
	idx := strings.Index(text, "\n{")
	if idx < 0 {
		t.Fatalf("no structured error JSON in %q", text)
	}
	var se StructuredError
	if err := json.Unmarshal([]byte(text[idx+1:]), &se); err != nil {
		t.Fatalf("parse structured error %q: %v", text[idx+1:], err)
	}
	return se
}

// domcovPayload parses the JSON object following the summary line of a success response.
func domcovPayload(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	if result.IsError {
		t.Fatalf("expected a success response, got error: %s", firstText(result))
	}
	text := firstText(result)
	nl := strings.Index(text, "\n")
	if nl < 0 {
		t.Fatalf("no JSON payload after summary line: %q", text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text[nl+1:]), &out); err != nil {
		t.Fatalf("parse payload %q: %v", text[nl+1:], err)
	}
	return out
}

func domcovSummary(t *testing.T, resp JSONRPCResponse) string {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	text := firstText(result)
	if nl := strings.Index(text, "\n"); nl >= 0 {
		return text[:nl]
	}
	return text
}

// domcovJSONMap unmarshals raw JSON into a map, failing the test on error.
func domcovJSONMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse JSON %q: %v", string(raw), err)
	}
	return out
}

// domcovApplyOpts materialises StructuredError option funcs so callers can assert on them.
func domcovApplyOpts(opts []func(*StructuredError)) StructuredError {
	var se StructuredError
	for _, o := range opts {
		o(&se)
	}
	return se
}

// ---------------------------------------------------------------------------
// interact_dom_validation.go — selector requirement
// ---------------------------------------------------------------------------

func TestDOMPrimitive_MissingSelectorNamesAllThreeWaysToIdentifyAnElement(t *testing.T) {
	t.Parallel()
	resp, failed := validateDOMSelectorRequirement(domcovReq(), "click", DOMPrimitiveParams{})
	if !failed {
		t.Fatal("click without selector/element_id/index must be rejected")
	}
	se := domcovError(t, resp)
	if se.ErrorCode != ErrMissingParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrMissingParam)
	}
	if se.Param != "selector" {
		t.Errorf("param = %q, want selector", se.Param)
	}
	if se.Message != "Required parameter 'selector', 'element_id', or 'index' is missing" {
		t.Errorf("message = %q", se.Message)
	}
	// The playbook must mention list_interactive, which is the only way to obtain index/element_id.
	if !strings.Contains(se.RecoveryPlaybook, "list_interactive") {
		t.Errorf("recovery_playbook must point at list_interactive, got %q", se.RecoveryPlaybook)
	}
}

func TestDOMPrimitive_SelectorRequirementSatisfiedByAnyIdentifier(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		action string
		params DOMPrimitiveParams
	}{
		{"css selector", "click", DOMPrimitiveParams{Selector: "#go"}},
		{"element id", "click", DOMPrimitiveParams{ElementID: "el-7"}},
		// key_press / wait_for act on the document, not a specific node.
		{"selector-optional key_press", "key_press", DOMPrimitiveParams{}},
		{"selector-optional wait_for", "wait_for", DOMPrimitiveParams{}},
		{"selector-optional confirm_top_dialog", "confirm_top_dialog", DOMPrimitiveParams{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, failed := validateDOMSelectorRequirement(domcovReq(), tc.action, tc.params); failed {
				t.Fatalf("%s/%s must pass the selector requirement", tc.action, tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// interact_dom_validation.go — wait_for conditions
// ---------------------------------------------------------------------------

func TestWaitFor_NoConditionIsRejectedWithTheThreeAllowedConditions(t *testing.T) {
	t.Parallel()
	resp, failed := validateWaitForConditions(domcovReq(), "wait_for", DOMPrimitiveParams{})
	if !failed {
		t.Fatal("wait_for with no condition must be rejected")
	}
	se := domcovError(t, resp)
	if se.ErrorCode != ErrMissingParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrMissingParam)
	}
	if se.Message != "wait_for requires at least one condition: selector, text, or url_contains" {
		t.Errorf("message = %q", se.Message)
	}
	if se.Param != "selector" {
		t.Errorf("param = %q, want selector", se.Param)
	}
}

func TestWaitFor_ConditionsAreMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		params DOMPrimitiveParams
	}{
		{"selector and text", DOMPrimitiveParams{Selector: "#a", Text: "done"}},
		{"selector and url", DOMPrimitiveParams{Selector: "#a", URLContains: "/next"}},
		{"text and url", DOMPrimitiveParams{Text: "done", URLContains: "/next"}},
		{"all three", DOMPrimitiveParams{Selector: "#a", Text: "done", URLContains: "/next"}},
		// absent counts as the selector condition even without a selector string.
		{"absent and text", DOMPrimitiveParams{Absent: true, Text: "done"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, failed := validateWaitForConditions(domcovReq(), "wait_for", tc.params)
			if !failed {
				t.Fatalf("%s must be rejected as mutually exclusive", tc.name)
			}
			se := domcovError(t, resp)
			if se.ErrorCode != ErrInvalidParam {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidParam)
			}
			if !strings.Contains(se.Message, "mutually exclusive") {
				t.Errorf("message = %q, want a mutual-exclusion complaint", se.Message)
			}
		})
	}
}

func TestWaitFor_AbsentWithoutSelectorIsRejected(t *testing.T) {
	t.Parallel()
	resp, failed := validateWaitForConditions(domcovReq(), "wait_for", DOMPrimitiveParams{Absent: true})
	if !failed {
		t.Fatal("wait_for absent with no selector must be rejected")
	}
	se := domcovError(t, resp)
	if se.ErrorCode != ErrMissingParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrMissingParam)
	}
	if se.Message != "wait_for with absent requires a selector" {
		t.Errorf("message = %q", se.Message)
	}
}

func TestWaitFor_SingleConditionIsAccepted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		params DOMPrimitiveParams
	}{
		{"selector only", DOMPrimitiveParams{Selector: "#a"}},
		{"element_id only", DOMPrimitiveParams{ElementID: "el-1"}},
		{"text only", DOMPrimitiveParams{Text: "Saved"}},
		{"url_contains only", DOMPrimitiveParams{URLContains: "/done"}},
		{"absent with selector", DOMPrimitiveParams{Selector: "#spinner", Absent: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, failed := validateWaitForConditions(domcovReq(), "wait_for", tc.params); failed {
				t.Fatalf("%s must be accepted", tc.name)
			}
		})
	}
}

func TestWaitFor_ValidationOnlyAppliesToWaitForAction(t *testing.T) {
	t.Parallel()
	// click with no wait conditions at all must not trip the wait_for validator.
	if _, failed := validateWaitForConditions(domcovReq(), "click", DOMPrimitiveParams{}); failed {
		t.Fatal("wait_for validation must not apply to click")
	}
	// ...nor should a click carrying several would-be conditions be rejected.
	both := DOMPrimitiveParams{Selector: "#a", Text: "x", URLContains: "/y"}
	if _, failed := validateWaitForConditions(domcovReq(), "type", both); failed {
		t.Fatal("wait_for mutual exclusion must not apply to type")
	}
}

// ---------------------------------------------------------------------------
// interact_dom_validation.go — per-action required params
// ---------------------------------------------------------------------------

func TestDOMActionParams_ActionSpecificRequiredParamIsNamedInTheError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		action    string
		wantParam string
		wantMsg   string
	}{
		{"type", "text", "Required parameter 'text' is missing for type action"},
		{"paste", "text", "Required parameter 'text' is missing for paste action"},
		{"select", "value", "Required parameter 'value' is missing for select action"},
		{"get_attribute", "name", "Required parameter 'name' is missing for get_attribute action"},
		{"set_attribute", "name", "Required parameter 'name' is missing for set_attribute action"},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			t.Parallel()
			resp, failed := ValidateDOMActionParams(domcovReq(), tc.action, "", "", "")
			if !failed {
				t.Fatalf("%s without its required param must be rejected", tc.action)
			}
			se := domcovError(t, resp)
			if se.ErrorCode != ErrMissingParam {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrMissingParam)
			}
			if se.Param != tc.wantParam {
				t.Errorf("param = %q, want %q", se.Param, tc.wantParam)
			}
			if se.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", se.Message, tc.wantMsg)
			}
		})
	}
}

func TestDOMActionParams_SuppliedValueSatisfiesTheRule(t *testing.T) {
	t.Parallel()
	// Each action reads only its own field: supplying the wrong one must still fail.
	if _, failed := ValidateDOMActionParams(domcovReq(), "type", "hello", "", ""); failed {
		t.Error("type with text must pass")
	}
	if _, failed := ValidateDOMActionParams(domcovReq(), "type", "", "hello", ""); !failed {
		t.Error("type must not be satisfied by 'value'")
	}
	if _, failed := ValidateDOMActionParams(domcovReq(), "select", "", "opt-1", ""); failed {
		t.Error("select with value must pass")
	}
	if _, failed := ValidateDOMActionParams(domcovReq(), "get_attribute", "", "", "href"); failed {
		t.Error("get_attribute with name must pass")
	}
	// Actions with no rule are always allowed through.
	if _, failed := ValidateDOMActionParams(domcovReq(), "click", "", "", ""); failed {
		t.Error("click has no required param and must pass")
	}
}

func TestDOMActionContextOptions_SelectorOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	withSel := domcovApplyOpts(domActionContextOptions("click", "#submit"))
	if withSel.Action != "click" || withSel.Selector != "#submit" {
		t.Errorf("action=%q selector=%q, want click/#submit", withSel.Action, withSel.Selector)
	}
	noSel := domcovApplyOpts(domActionContextOptions("key_press", ""))
	if noSel.Action != "key_press" {
		t.Errorf("action = %q, want key_press", noSel.Action)
	}
	if noSel.Selector != "" {
		t.Errorf("selector = %q, want empty when no selector was supplied", noSel.Selector)
	}
}

// ---------------------------------------------------------------------------
// interact_dom_params.go
// ---------------------------------------------------------------------------

func TestParseDOMPrimitiveParams_OptionalPointersDistinguishAbsentFromZero(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"selector":"#email","scope_selector":"form","element_id":"el-3",
		"index":0,"nth":2,"index_generation":"gen_9",
		"text":"hi","value":"v1","direction":"down","clear":true,
		"checked":false,"name":"href","timeout_ms":250,"tab_id":42,
		"analyze":true,"x":0,"y":12.5,"url_contains":"/done","absent":true,"structured":true
	}`)
	p, err := ParseDOMPrimitiveParams(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Selector != "#email" || p.ScopeSelector != "form" || p.ElementID != "el-3" {
		t.Errorf("selector=%q scope=%q element_id=%q", p.Selector, p.ScopeSelector, p.ElementID)
	}
	// index:0 and x:0 and checked:false must be *present*, not indistinguishable from omitted.
	if p.Index == nil || *p.Index != 0 {
		t.Errorf("index = %v, want pointer to 0", p.Index)
	}
	if p.X == nil || *p.X != 0 {
		t.Errorf("x = %v, want pointer to 0", p.X)
	}
	if p.Checked == nil || *p.Checked {
		t.Errorf("checked = %v, want pointer to false", p.Checked)
	}
	if p.Nth == nil || *p.Nth != 2 {
		t.Errorf("nth = %v, want pointer to 2", p.Nth)
	}
	if p.Y == nil || *p.Y != 12.5 {
		t.Errorf("y = %v, want pointer to 12.5", p.Y)
	}
	if p.IndexGen != "gen_9" || p.Text != "hi" || p.Value != "v1" || p.Direction != "down" || p.Name != "href" {
		t.Errorf("string fields wrong: %+v", p)
	}
	if p.TimeoutMs != 250 || p.TabID != 42 {
		t.Errorf("timeout_ms=%d tab_id=%d, want 250/42", p.TimeoutMs, p.TabID)
	}
	if !p.Clear || !p.Analyze || !p.Absent || !p.Structured {
		t.Errorf("bool flags wrong: %+v", p)
	}
	if p.URLContains != "/done" {
		t.Errorf("url_contains = %q", p.URLContains)
	}
}

func TestParseDOMPrimitiveParams_OmittedOptionalsStayNil(t *testing.T) {
	t.Parallel()
	p, err := ParseDOMPrimitiveParams(json.RawMessage(`{"selector":"#a"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Index != nil || p.Nth != nil || p.X != nil || p.Y != nil || p.Checked != nil {
		t.Fatalf("omitted optional pointers must stay nil, got %+v", p)
	}
}

func TestParseDOMPrimitiveParams_MalformedJSONYieldsZeroValueNotPartialParse(t *testing.T) {
	t.Parallel()
	p, err := ParseDOMPrimitiveParams(json.RawMessage(`{"selector":"#a", "tab_id":`))
	if err == nil {
		t.Fatal("truncated JSON must return an error")
	}
	if p.Selector != "" {
		t.Errorf("selector = %q, want empty — a failed parse must not leak partial fields", p.Selector)
	}
}

func TestParseHardwareClickParams_MissingCoordinatesAreNilNotZero(t *testing.T) {
	t.Parallel()
	p, err := parseHardwareClickParams(json.RawMessage(`{"tab_id":5}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.X != nil || p.Y != nil {
		t.Fatalf("x/y must be nil when absent, got %v/%v", p.X, p.Y)
	}
	if p.TabID != 5 {
		t.Errorf("tab_id = %d, want 5", p.TabID)
	}

	got, err := parseHardwareClickParams(json.RawMessage(`{"x":0,"y":0}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.X == nil || got.Y == nil || *got.X != 0 || *got.Y != 0 {
		t.Fatalf("explicit 0,0 must survive as non-nil zeroes, got %v/%v", got.X, got.Y)
	}

	if _, err := parseHardwareClickParams(json.RawMessage(`not json`)); err == nil {
		t.Error("malformed JSON must return an error")
	}
}

func TestUpdateArgsSelector_InjectsSelectorAndKeepsSiblingKeys(t *testing.T) {
	t.Parallel()
	out := updateArgsSelector(json.RawMessage(`{"index":3,"tab_id":9,"text":"hi"}`), "#password")
	m := domcovJSONMap(t, out)
	if m["selector"] != "#password" {
		t.Errorf("selector = %v, want #password", m["selector"])
	}
	if m["index"] != float64(3) || m["tab_id"] != float64(9) || m["text"] != "hi" {
		t.Errorf("sibling keys lost: %v", m)
	}
}

func TestUpdateArgsSelector_OverwritesAnExistingSelector(t *testing.T) {
	t.Parallel()
	out := updateArgsSelector(json.RawMessage(`{"selector":"#old"}`), "#new")
	if got := domcovJSONMap(t, out)["selector"]; got != "#new" {
		t.Errorf("selector = %v, want #new", got)
	}
}

func TestUpdateArgsSelector_UnparseableArgsPassThroughUnchanged(t *testing.T) {
	t.Parallel()
	in := json.RawMessage(`[1,2,3]`)
	if got := string(updateArgsSelector(in, "#x")); got != `[1,2,3]` {
		t.Errorf("got %q, want the original args back when they are not a JSON object", got)
	}
}

// ---------------------------------------------------------------------------
// interact_dom.go — argument normalisation
// ---------------------------------------------------------------------------

func TestNormalizeDOMActionArgs_CanonicalActionIsAddedAlongsideWhat(t *testing.T) {
	t.Parallel()
	// The extension dispatches on "action"; "what" is the user-facing name and must survive.
	m := domcovJSONMap(t, normalizeDOMActionArgs(json.RawMessage(`{"what":"click","selector":"#go"}`), "click"))
	if m["action"] != "click" {
		t.Errorf("action = %v, want click", m["action"])
	}
	if m["what"] != "click" {
		t.Errorf("what = %v, want click (preserved)", m["what"])
	}
	if m["selector"] != "#go" {
		t.Errorf("selector = %v, want #go", m["selector"])
	}
}

func TestNormalizeDOMActionArgs_UnparseableArgsBecomeActionOnlyPayload(t *testing.T) {
	t.Parallel()
	m := domcovJSONMap(t, normalizeDOMActionArgs(json.RawMessage(`garbage`), "hover"))
	if len(m) != 1 || m["action"] != "hover" {
		t.Errorf("got %v, want exactly {action: hover}", m)
	}
}

func TestNormalizeDOMActionArgs_AnnotationRectIsPromotedToScopeRect(t *testing.T) {
	t.Parallel()
	args := json.RawMessage(`{"annotation_rect":{"x":10,"y":20,"width":30,"height":40}}`)
	m := domcovJSONMap(t, normalizeDOMActionArgs(args, "list_interactive"))
	rect, ok := m["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("scope_rect missing: %v", m)
	}
	if rect["x"] != float64(10) || rect["y"] != float64(20) || rect["width"] != float64(30) || rect["height"] != float64(40) {
		t.Errorf("scope_rect = %v, want a copy of annotation_rect", rect)
	}
}

func TestNormalizeDOMActionArgs_ExplicitScopeRectBeatsAnnotationRect(t *testing.T) {
	t.Parallel()
	args := json.RawMessage(`{"scope_rect":{"x":1},"annotation_rect":{"x":99}}`)
	m := domcovJSONMap(t, normalizeDOMActionArgs(args, "click"))
	rect := m["scope_rect"].(map[string]any)
	if rect["x"] != float64(1) {
		t.Errorf("scope_rect.x = %v, want 1 — an explicit scope_rect must not be overwritten", rect["x"])
	}
}

func TestNormalizeDOMActionArgs_NearRadiusExpandsToASquareScopeRect(t *testing.T) {
	t.Parallel()
	// #448: near_x/near_y/near_radius describe a circle; the wire format is the bounding box.
	args := json.RawMessage(`{"near_x":100,"near_y":50,"near_radius":20}`)
	m := domcovJSONMap(t, normalizeDOMActionArgs(args, "list_interactive"))
	rect, ok := m["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("scope_rect missing: %v", m)
	}
	if rect["x"] != float64(80) || rect["y"] != float64(30) {
		t.Errorf("origin = (%v,%v), want (80,30)", rect["x"], rect["y"])
	}
	if rect["width"] != float64(40) || rect["height"] != float64(40) {
		t.Errorf("size = (%v,%v), want (40,40)", rect["width"], rect["height"])
	}
}

func TestNormalizeDOMActionArgs_IncompleteOrZeroNearParamsProduceNoScopeRect(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"zero radius":     `{"near_x":10,"near_y":10,"near_radius":0}`,
		"negative radius": `{"near_x":10,"near_y":10,"near_radius":-5}`,
		"missing y":       `{"near_x":10,"near_radius":5}`,
		"missing x":       `{"near_y":10,"near_radius":5}`,
		"missing radius":  `{"near_x":10,"near_y":10}`,
		"non-numeric x":   `{"near_x":"10","near_y":10,"near_radius":5}`,
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := domcovJSONMap(t, normalizeDOMActionArgs(json.RawMessage(args), "click"))
			if _, ok := m["scope_rect"]; ok {
				t.Errorf("scope_rect must not be synthesised for %s, got %v", name, m["scope_rect"])
			}
		})
	}
}

func TestToFloat64_AcceptsOnlyNumericJSONShapes(t *testing.T) {
	t.Parallel()
	if v, ok := toFloat64(float64(2.5)); !ok || v != 2.5 {
		t.Errorf("float64: got %v/%v", v, ok)
	}
	if v, ok := toFloat64(7); !ok || v != 7 {
		t.Errorf("int: got %v/%v", v, ok)
	}
	if v, ok := toFloat64(json.Number("3.5")); !ok || v != 3.5 {
		t.Errorf("json.Number: got %v/%v", v, ok)
	}
	if _, ok := toFloat64(json.Number("abc")); ok {
		t.Error("unparseable json.Number must be rejected")
	}
	for _, bad := range []any{nil, "5", true, []any{1}} {
		if v, ok := toFloat64(bad); ok {
			t.Errorf("toFloat64(%#v) = %v/true, want rejected", bad, v)
		}
	}
}

// ---------------------------------------------------------------------------
// interact_dom.go — HandleDOMPrimitive
// ---------------------------------------------------------------------------

func TestHandleDOMPrimitive_MalformedJSONIsRejectedBeforeAnyGuardRuns(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(`{"selector":`), "click")
	se := domcovError(t, resp)
	if se.ErrorCode != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidJSON)
	}
	if !strings.Contains(se.Message, "Invalid JSON arguments") {
		t.Errorf("message = %q", se.Message)
	}
	if len(hs.guardCalls) != 0 {
		t.Errorf("guards ran on unparseable args: %v", hs.guardCalls)
	}
	if len(hs.queries) != 0 {
		t.Errorf("a malformed request must never be enqueued: %+v", hs.queries)
	}
}

func TestHandleDOMPrimitive_QueuesDOMActionAndRecordsTheReproductionStep(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	args := json.RawMessage(`{"what":"type","selector":"#email","text":"a@b.c","value":"v","tab_id":11}`)
	resp := hs.h.HandleDOMPrimitive(domcovReq(), args, "type")

	q := hs.domcovOnlyQuery()
	if q.Type != "dom_action" {
		t.Errorf("query type = %q, want dom_action", q.Type)
	}
	if q.TabID != 11 {
		t.Errorf("tab_id = %d, want 11", q.TabID)
	}
	if q.Params["action"] != "type" || q.Params["selector"] != "#email" || q.Params["text"] != "a@b.c" {
		t.Errorf("query params = %v", q.Params)
	}
	if !strings.HasPrefix(q.CorrelationID, "dom_type_") {
		t.Errorf("correlation_id = %q, want a dom_type_ prefix", q.CorrelationID)
	}
	if len(hs.primitives) != 1 {
		t.Fatalf("expected 1 recorded primitive, got %+v", hs.primitives)
	}
	want := domcovPrimitive{"type", "#email", "a@b.c", "v"}
	if hs.primitives[0] != want {
		t.Errorf("recorded primitive = %+v, want %+v", hs.primitives[0], want)
	}
	if got := domcovSummary(t, resp); got != "type queued" {
		t.Errorf("queued message = %q, want \"type queued\"", got)
	}
}

func TestHandleDOMPrimitive_ClickWithCoordinatesEscalatesToHardwareCDPClick(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	args := json.RawMessage(`{"selector":"#ignored","x":120.5,"y":40,"tab_id":3}`)
	hs.h.HandleDOMPrimitive(domcovReq(), args, "click")

	q := hs.domcovOnlyQuery()
	if q.Type != "cdp_action" {
		t.Fatalf("query type = %q, want cdp_action — x/y must bypass the DOM path", q.Type)
	}
	if q.Params["action"] != "click" || q.Params["x"] != float64(120.5) || q.Params["y"] != float64(40) {
		t.Errorf("cdp params = %v", q.Params)
	}
	// The selector must not travel with a coordinate click; CDP dispatches on pixels only.
	if _, present := q.Params["selector"]; present {
		t.Errorf("cdp params must not carry a selector: %v", q.Params)
	}
	if q.TabID != 3 {
		t.Errorf("tab_id = %d, want 3", q.TabID)
	}
	if len(hs.primitives) != 0 {
		t.Errorf("a coordinate click is not a DOM primitive: %+v", hs.primitives)
	}
	ai := hs.domcovOnlyAI()
	if ai.Action != "click" || ai.Extra["method"] != "cdp" {
		t.Errorf("recorded AI action = %+v, want click via cdp", ai)
	}
}

func TestHandleDOMPrimitive_OnlyBothCoordinatesTriggerTheCDPPath(t *testing.T) {
	t.Parallel()
	for name, args := range map[string]string{
		"x only": `{"selector":"#a","x":10}`,
		"y only": `{"selector":"#a","y":10}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(args), "click")
			if q := hs.domcovOnlyQuery(); q.Type != "dom_action" {
				t.Errorf("query type = %q, want dom_action when only one coordinate is given", q.Type)
			}
		})
	}
}

func TestHandleDOMPrimitive_GuardRejectionCarriesActionAndSelectorContext(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockPilot = true
	resp := hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(`{"selector":"#go"}`), "click")

	se := domcovError(t, resp)
	// The blocked-guard error must say which action and selector were attempted,
	// otherwise the agent cannot tell which of a batch of steps was refused.
	if se.Action != "click" {
		t.Errorf("action = %q, want click", se.Action)
	}
	if se.Selector != "#go" {
		t.Errorf("selector = %q, want #go", se.Selector)
	}
	if len(hs.queries) != 0 {
		t.Errorf("a blocked command must not be enqueued: %+v", hs.queries)
	}
}

func TestHandleDOMPrimitive_UnknownIndexIsRejectedWithTabScopedGuidance(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(`{"index":5,"tab_id":3}`), "click")
	se := domcovError(t, resp)
	if se.ErrorCode != ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidParam)
	}
	if !strings.Contains(se.Message, "Element index 5 not found for tab_id=3") {
		t.Errorf("message = %q, want it to name the index and tab", se.Message)
	}
	if len(hs.queries) != 0 {
		t.Errorf("an unresolved index must not be enqueued: %+v", hs.queries)
	}
}

func TestHandleDOMPrimitive_ResolvedIndexIsRewrittenIntoTheDispatchedSelector(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.elementIndexRegistry.store("domcov-client", 4, "gen_1", map[int]string{2: "button.save"})

	hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(`{"index":2,"tab_id":4}`), "click")

	q := hs.domcovOnlyQuery()
	if q.Params["selector"] != "button.save" {
		t.Fatalf("dispatched selector = %v, want button.save", q.Params["selector"])
	}
	if len(hs.primitives) != 1 || hs.primitives[0].Selector != "button.save" {
		t.Errorf("reproduction must record the resolved selector, got %+v", hs.primitives)
	}
}

func TestHandleDOMPrimitive_ActionValidationRunsBeforeDispatch(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(`{"selector":"#in"}`), "type")
	se := domcovError(t, resp)
	if se.Param != "text" {
		t.Errorf("param = %q, want text", se.Param)
	}
	if len(hs.queries) != 0 {
		t.Errorf("type without text must not reach the extension: %+v", hs.queries)
	}
}

// ---------------------------------------------------------------------------
// interact_dom_validation.go — resolveDOMSelectorFromIndex
// ---------------------------------------------------------------------------

func TestResolveDOMSelectorFromIndex_ExplicitSelectorSuppressesIndexLookup(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.elementIndexRegistry.store("domcov-client", 0, "gen_1", map[int]string{1: "#from-index"})

	params := DOMPrimitiveParams{Selector: "#explicit", Index: new(int)}
	*params.Index = 1
	args := json.RawMessage(`{"selector":"#explicit","index":1}`)

	out, _, failed := hs.h.resolveDOMSelectorFromIndex(domcovReq(), args, &params)
	if failed {
		t.Fatal("an explicit selector must not fail index resolution")
	}
	if params.Selector != "#explicit" {
		t.Errorf("selector = %q, want the caller's explicit selector to win", params.Selector)
	}
	if string(out) != string(args) {
		t.Errorf("args = %s, want them untouched", out)
	}
}

func TestResolveDOMSelectorFromIndex_NoIndexIsANoOp(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	params := DOMPrimitiveParams{}
	args := json.RawMessage(`{"tab_id":1}`)
	out, _, failed := hs.h.resolveDOMSelectorFromIndex(domcovReq(), args, &params)
	if failed || params.Selector != "" || string(out) != string(args) {
		t.Fatalf("no-index request must pass through untouched: failed=%v selector=%q args=%s", failed, params.Selector, out)
	}
}

func TestResolveDOMSelectorFromIndex_StaleGenerationReportsBothGenerations(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.elementIndexRegistry.store("domcov-client", 0, "gen_new", map[int]string{1: "#a"})

	idx := 1
	params := DOMPrimitiveParams{Index: &idx, IndexGen: "gen_old"}
	_, resp, failed := hs.h.resolveDOMSelectorFromIndex(domcovReq(), json.RawMessage(`{"index":1}`), &params)
	if !failed {
		t.Fatal("a stale index_generation must be rejected")
	}
	se := domcovError(t, resp)
	if se.ErrorCode != ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidParam)
	}
	if !strings.Contains(se.Message, `expected "gen_old"`) || !strings.Contains(se.Message, `latest "gen_new"`) {
		t.Errorf("message = %q, want both the sent and current generation", se.Message)
	}
}

// ---------------------------------------------------------------------------
// interact_dom_click.go
// ---------------------------------------------------------------------------

func TestHandleHardwareClick_EachMissingCoordinateIsNamedIndividually(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, args, wantParam, wantMsg string
	}{
		{"no x", `{"y":10}`, "x", "Required parameter 'x' is missing"},
		{"no y", `{"x":10}`, "y", "Required parameter 'y' is missing"},
		{"neither", `{}`, "x", "Required parameter 'x' is missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			se := domcovError(t, hs.h.HandleHardwareClick(domcovReq(), json.RawMessage(tc.args)))
			if se.ErrorCode != ErrMissingParam {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrMissingParam)
			}
			if se.Param != tc.wantParam {
				t.Errorf("param = %q, want %q", se.Param, tc.wantParam)
			}
			if se.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", se.Message, tc.wantMsg)
			}
			if len(hs.queries) != 0 {
				t.Errorf("must not enqueue: %+v", hs.queries)
			}
		})
	}
}

func TestHandleHardwareClick_MalformedJSONIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleHardwareClick(domcovReq(), json.RawMessage(`{"x":`)))
	if se.ErrorCode != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidJSON)
	}
}

func TestHandleHardwareClick_DispatchesCDPMouseEventAtTheGivenPixel(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleHardwareClick(domcovReq(), json.RawMessage(`{"x":15.5,"y":0,"tab_id":8}`))

	q := hs.domcovOnlyQuery()
	if q.Type != "cdp_action" {
		t.Errorf("query type = %q, want cdp_action", q.Type)
	}
	if q.Params["x"] != float64(15.5) || q.Params["y"] != float64(0) {
		t.Errorf("coords = (%v,%v), want (15.5,0) — y=0 must survive as a real coordinate", q.Params["x"], q.Params["y"])
	}
	if q.TabID != 8 {
		t.Errorf("tab_id = %d, want 8", q.TabID)
	}
	if !strings.HasPrefix(q.CorrelationID, "cdp_click_") {
		t.Errorf("correlation_id = %q, want cdp_click_ prefix", q.CorrelationID)
	}
	ai := hs.domcovOnlyAI()
	if ai.Action != "hardware_click" {
		t.Errorf("recorded action = %q, want hardware_click", ai.Action)
	}
	if ai.Extra["method"] != "cdp" || ai.Extra["x"] != 15.5 {
		t.Errorf("recorded extra = %v", ai.Extra)
	}
	if got := domcovSummary(t, resp); got != "hardware_click queued" {
		t.Errorf("queued message = %q", got)
	}
}

// ---------------------------------------------------------------------------
// interact_upload.go
// ---------------------------------------------------------------------------

func domcovTempFile(t *testing.T, name string, size int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestUpload_ParameterValidationRejectionsNameTheOffendingParam(t *testing.T) {
	t.Parallel()
	abs := filepath.Join(t.TempDir(), "f.txt")
	cases := []struct {
		name      string
		params    uploadParams
		wantCode  string
		wantParam string
		wantMsg   string
	}{
		{
			"no file_path", uploadParams{Selector: "#f"},
			ErrMissingParam, "file_path", "Required parameter 'file_path' is missing",
		},
		{
			"no selector and no api_endpoint", uploadParams{FilePath: abs},
			ErrMissingParam, "selector",
			"Required parameter 'selector' is missing. Provide a CSS selector for the file input element, or use 'api_endpoint' for direct API uploads.",
		},
		{
			"relative path", uploadParams{Selector: "#f", FilePath: "rel/file.txt"},
			ErrPathNotAllowed, "file_path",
			"file_path must be an absolute path. Relative paths are not allowed for security.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := validateUploadParams(domcovReq(), tc.params)
			if resp == nil {
				t.Fatalf("%s must be rejected", tc.name)
			}
			se := domcovError(t, *resp)
			if se.ErrorCode != tc.wantCode {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, tc.wantCode)
			}
			if se.Param != tc.wantParam {
				t.Errorf("param = %q, want %q", se.Param, tc.wantParam)
			}
			if se.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", se.Message, tc.wantMsg)
			}
		})
	}
}

func TestUpload_APIEndpointAloneSatisfiesTheSelectorRequirement(t *testing.T) {
	t.Parallel()
	abs := filepath.Join(t.TempDir(), "f.txt")
	got := validateUploadParams(domcovReq(), uploadParams{FilePath: abs, APIEndpoint: "https://host/upload"})
	if got != nil {
		t.Fatalf("api_endpoint must stand in for selector, got %s", firstText(parseToolResult(t, *got)))
	}
}

func TestUpload_FileChecksDistinguishMissingFromDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, resp := validateUploadFile(domcovReq(), filepath.Join(dir, "nope.txt"))
	if resp == nil {
		t.Fatal("a non-existent file must be rejected")
	}
	se := domcovError(t, *resp)
	if se.ErrorCode != ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidParam)
	}
	if !strings.HasPrefix(se.Message, "File not found: ") {
		t.Errorf("message = %q, want a File not found message", se.Message)
	}

	_, dirResp := validateUploadFile(domcovReq(), dir)
	if dirResp == nil {
		t.Fatal("a directory must be rejected")
	}
	dse := domcovError(t, *dirResp)
	if dse.ErrorCode != ErrInvalidParam || !strings.HasPrefix(dse.Message, "Path is a directory, not a file: ") {
		t.Errorf("directory rejection = %q / %q", dse.ErrorCode, dse.Message)
	}
	if dse.Param != "file_path" {
		t.Errorf("param = %q, want file_path", dse.Param)
	}
}

func TestUpload_ValidFileReturnsItsInfo(t *testing.T) {
	t.Parallel()
	path := domcovTempFile(t, "clip.mp4", 1234)
	info, resp := validateUploadFile(domcovReq(), path)
	if resp != nil {
		t.Fatalf("a readable file must pass: %s", firstText(parseToolResult(t, *resp)))
	}
	if info == nil || info.Size() != 1234 {
		t.Fatalf("info = %v, want a 1234-byte stat", info)
	}
}

func TestUpload_ParameterValidationHappensBeforeGuards(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockPilot = true
	// Pilot is off AND file_path is missing: the argument error is the actionable
	// one, so it must win — otherwise the agent fixes pilot mode and fails again.
	se := domcovError(t, hs.up.HandleUpload(domcovReq(), json.RawMessage(`{"selector":"#f"}`)))
	if se.ErrorCode != ErrMissingParam || se.Param != "file_path" {
		t.Errorf("got %q/%q, want missing_param/file_path", se.ErrorCode, se.Param)
	}
	if len(hs.guardCalls) != 0 {
		t.Errorf("guards must not run before parameter validation: %v", hs.guardCalls)
	}
}

func TestUpload_FileStatHappensAfterGuardsSoPilotErrorsSurfaceFirst(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockExt = true
	args := json.RawMessage(`{"selector":"#f","file_path":"` + filepath.Join(t.TempDir(), "missing.txt") + `"}`)
	se := domcovError(t, hs.up.HandleUpload(domcovReq(), args))
	if se.ErrorCode != ErrCodePilotDisabled {
		t.Errorf("error_code = %q, want the extension guard rejection", se.ErrorCode)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestUpload_QueuedPayloadCarriesFileMetadataAndDefaults(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	path := domcovTempFile(t, "notes.txt", 42)
	args := json.RawMessage(`{"selector":"#Filedata","file_path":"` + path + `","submit":true}`)

	resp := hs.up.HandleUpload(domcovReq(), args)

	q := hs.domcovOnlyQuery()
	if q.Type != "upload" {
		t.Errorf("query type = %q, want upload", q.Type)
	}
	if q.Timeout != 10*time.Minute {
		t.Errorf("enqueue timeout = %v, want 10m — uploads must not use the short async budget", q.Timeout)
	}
	if q.Params["action"] != "upload" || q.Params["selector"] != "#Filedata" {
		t.Errorf("payload = %v", q.Params)
	}
	if q.Params["file_name"] != "notes.txt" || q.Params["file_size"] != float64(42) {
		t.Errorf("file metadata = %v", q.Params)
	}
	if q.Params["mime_type"] != "text/plain" {
		t.Errorf("mime_type = %v, want text/plain (derived from the extension)", q.Params["mime_type"])
	}
	if q.Params["progress_tier"] != "simple" {
		t.Errorf("progress_tier = %v, want simple for a 42-byte file", q.Params["progress_tier"])
	}
	if q.Params["submit"] != true {
		t.Errorf("submit = %v, want true", q.Params["submit"])
	}
	if q.Params["escalation_timeout_ms"] != float64(5000) {
		t.Errorf("escalation_timeout_ms = %v, want the 5000ms default", q.Params["escalation_timeout_ms"])
	}
	if _, present := q.Params["api_endpoint"]; present {
		t.Errorf("api_endpoint must be omitted when unset: %v", q.Params)
	}

	payload := domcovPayload(t, resp)
	if payload["status"] != "queued" || payload["file_name"] != "notes.txt" {
		t.Errorf("response payload = %v", payload)
	}
	if payload["correlation_id"] != q.CorrelationID {
		t.Errorf("response correlation_id = %v, want the enqueued %q", payload["correlation_id"], q.CorrelationID)
	}
	msg, _ := payload["message"].(string)
	if !strings.Contains(msg, q.CorrelationID) {
		t.Errorf("message must embed the correlation id for the observe() follow-up, got %q", msg)
	}

	ai := hs.domcovOnlyAI()
	if ai.Action != "upload" || ai.Extra["file_name"] != "notes.txt" || ai.Extra["selector"] != "#Filedata" {
		t.Errorf("recorded AI action = %+v", ai)
	}
}

func TestUpload_ExplicitEscalationTimeoutAndAPIEndpointAreForwarded(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	path := domcovTempFile(t, "img.png", 8)
	args := json.RawMessage(`{"selector":"#f","file_path":"` + path + `","escalation_timeout_ms":1500,"api_endpoint":"https://host/u"}`)

	hs.up.HandleUpload(domcovReq(), args)

	q := hs.domcovOnlyQuery()
	if q.Params["escalation_timeout_ms"] != float64(1500) {
		t.Errorf("escalation_timeout_ms = %v, want 1500", q.Params["escalation_timeout_ms"])
	}
	if q.Params["api_endpoint"] != "https://host/u" {
		t.Errorf("api_endpoint = %v", q.Params["api_endpoint"])
	}
	if q.Params["mime_type"] != "image/png" {
		t.Errorf("mime_type = %v, want image/png", q.Params["mime_type"])
	}
}

func TestUpload_NonPositiveEscalationTimeoutFallsBackToTheDefault(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	path := domcovTempFile(t, "a.bin", 1)
	args := json.RawMessage(`{"selector":"#f","file_path":"` + path + `","escalation_timeout_ms":-1}`)
	hs.up.HandleUpload(domcovReq(), args)
	if got := hs.domcovOnlyQuery().Params["escalation_timeout_ms"]; got != float64(5000) {
		t.Errorf("escalation_timeout_ms = %v, want 5000", got)
	}
}

func TestUpload_UnknownExtensionFallsBackToOctetStream(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	path := domcovTempFile(t, "payload.weirdext", 3)
	hs.up.HandleUpload(domcovReq(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`"}`))
	if got := hs.domcovOnlyQuery().Params["mime_type"]; got != "application/octet-stream" {
		t.Errorf("mime_type = %v, want application/octet-stream", got)
	}
}

func TestUpload_BlockedEnqueueSkipsActionRecording(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockEnqueue = true
	path := domcovTempFile(t, "a.txt", 1)
	resp := hs.up.HandleUpload(domcovReq(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`"}`))

	if se := domcovError(t, resp); se.ErrorCode != ErrQueueFull {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrQueueFull)
	}
	if len(hs.aiActions) != 0 {
		t.Errorf("an upload that never queued must not be recorded as an action: %+v", hs.aiActions)
	}
}

func TestUpload_MalformedJSONIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.up.HandleUpload(domcovReq(), json.RawMessage(`{"selector":`)))
	if se.ErrorCode != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidJSON)
	}
}

// ---------------------------------------------------------------------------
// interact_browser_util_impl.go — insecure-proxy URL resolution
// ---------------------------------------------------------------------------

func TestResolveNavigateURL_OrdinaryURLsAreOnlyTrimmed(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	for _, in := range []string{"https://example.com/a", "  https://example.com/a  ", "about:blank", ""} {
		got, err := hs.h.ResolveNavigateURLImpl(in)
		if err != nil {
			t.Fatalf("ResolveNavigateURLImpl(%q): %v", in, err)
		}
		if got != strings.TrimSpace(in) {
			t.Errorf("ResolveNavigateURLImpl(%q) = %q, want the trimmed input", in, got)
		}
	}
	if hs.captureCalls != 0 {
		t.Errorf("a plain URL must not consult the capture store (%d calls)", hs.captureCalls)
	}
}

func TestResolveNavigateURL_InsecurePrefixRequiresInsecureProxyMode(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	_, err := hs.h.ResolveNavigateURLImpl("kaboom-insecure://https://self-signed.test/x")
	if err == nil {
		t.Fatal("kaboom-insecure:// must be refused while security_mode is normal")
	}
	if !strings.Contains(err.Error(), "security_mode=insecure_proxy") {
		t.Errorf("error = %q, want it to name the required security mode", err)
	}
}

func TestResolveNavigateURL_InsecurePrefixIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	// The prefix check lowercases, so an upper-case scheme must be caught too
	// rather than silently navigating to a literal "KABOOM-INSECURE://..." URL.
	if _, err := hs.h.ResolveNavigateURLImpl("KABOOM-INSECURE://https://x.test/"); err == nil {
		t.Fatal("upper-case kaboom-insecure prefix must be treated as the insecure scheme")
	}
}

func TestResolveNavigateURL_InsecureTargetsAreValidated(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, raw, wantErr string
	}{
		{"empty target", "kaboom-insecure://", "target URL is empty"},
		{"whitespace target", "kaboom-insecure://   ", "target URL is empty"},
		{"non-http scheme", "kaboom-insecure://ftp://host/f", "must use http or https"},
		{"scheme-less target", "kaboom-insecure://host/path", "must use http or https"},
		{"no host", "kaboom-insecure://http:///path", "must include host"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			hs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
			_, err := hs.h.ResolveNavigateURLImpl(tc.raw)
			if err == nil {
				t.Fatalf("%s must be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestResolveNavigateURL_InsecureTargetIsRewrittenThroughTheLocalProxy(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.listenPort = 9931
	hs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	got, err := hs.h.ResolveNavigateURLImpl("kaboom-insecure://https://self-signed.test/a?b=c&d=e")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := "http://127.0.0.1:9931/insecure-proxy?target=https%3A%2F%2Fself-signed.test%2Fa%3Fb%3Dc%26d%3De"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestResolveNavigateURL_InsecureProxyFallsBackToPort7890(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	hs.h.deps.GetListenPort = nil

	got, err := hs.h.ResolveNavigateURLImpl("kaboom-insecure://http://x.test/")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.HasPrefix(got, "http://127.0.0.1:7890/insecure-proxy?") {
		t.Errorf("got %q, want the default port 7890 when GetListenPort is unset", got)
	}
}

// ---------------------------------------------------------------------------
// interact_browser_util_impl.go — perf snapshot stashing
// ---------------------------------------------------------------------------

func TestStashPerfSnapshot_StoresTheBaselineKeyedByCorrelationID(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.cap.UpdateTrackedTab(1, "https://example.com/dashboard", "Dash")
	// NOTE: the lookup uses the URL *path*, so the stored snapshot must be keyed
	// by path for the baseline to be found.
	hs.cap.AddPerformanceSnapshots([]capture.PerformanceSnapshot{{URL: "/dashboard", Timestamp: "t0"}})

	hs.h.stashPerfSnapshotImpl("corr-1")

	snap, ok := hs.cap.GetAndDeleteBeforeSnapshot("corr-1")
	if !ok {
		t.Fatal("expected a before-snapshot to be stashed for corr-1")
	}
	if snap.Timestamp != "t0" {
		t.Errorf("stashed snapshot = %+v, want the tracked page's snapshot", snap)
	}
}

func TestStashPerfSnapshot_NoTrackedPathStashesNothing(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	// Tracked URL has no path component, so there is nothing to key a baseline on.
	hs.cap.UpdateTrackedTab(1, "https://example.com", "Home")
	hs.cap.AddPerformanceSnapshots([]capture.PerformanceSnapshot{{URL: "", Timestamp: "t0"}})

	hs.h.stashPerfSnapshotImpl("corr-2")

	if _, ok := hs.cap.GetAndDeleteBeforeSnapshot("corr-2"); ok {
		t.Error("no baseline should be stashed when the tracked URL has no path")
	}
}

func TestStashPerfSnapshot_UnknownPathStashesNothing(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.cap.UpdateTrackedTab(1, "https://example.com/never-measured", "X")
	hs.cap.AddPerformanceSnapshots([]capture.PerformanceSnapshot{{URL: "/other", Timestamp: "t0"}})

	hs.h.stashPerfSnapshotImpl("corr-3")

	if _, ok := hs.cap.GetAndDeleteBeforeSnapshot("corr-3"); ok {
		t.Error("a path with no recorded snapshot must not stash a baseline")
	}
}

// ---------------------------------------------------------------------------
// interact_browser_util_impl.go — subtitle + screenshot alias
// ---------------------------------------------------------------------------

func TestSubtitle_OmittedTextIsRejectedButEmptyStringIsAccepted(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleSubtitleImpl(domcovReq(), json.RawMessage(`{}`)))
	if se.ErrorCode != ErrMissingParam || se.Param != "text" {
		t.Errorf("got %q/%q, want missing_param/text", se.ErrorCode, se.Param)
	}
	if !strings.Contains(se.RecoveryPlaybook, "empty string to clear") {
		t.Errorf("playbook = %q, want it to explain that \"\" clears the subtitle", se.RecoveryPlaybook)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestSubtitle_EmptyTextReportsClearedAndNonEmptyReportsSet(t *testing.T) {
	t.Parallel()
	cases := []struct{ args, want string }{
		{`{"text":""}`, "Subtitle cleared"},
		{`{"text":"Step 3 of 4"}`, "Subtitle set"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			resp := hs.h.HandleSubtitleImpl(domcovReq(), json.RawMessage(tc.args))
			if got := domcovSummary(t, resp); got != tc.want {
				t.Errorf("summary = %q, want %q", got, tc.want)
			}
			q := hs.domcovOnlyQuery()
			if q.Type != "subtitle" {
				t.Errorf("query type = %q, want subtitle", q.Type)
			}
			// Subtitle is an overlay-only action: it must not be gated on guards.
			if len(hs.guardCalls) != 0 {
				t.Errorf("subtitle must not run guards, ran %v", hs.guardCalls)
			}
		})
	}
}

func TestSubtitle_MalformedJSONIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleSubtitleImpl(domcovReq(), json.RawMessage(`{"text":`)))
	if se.ErrorCode != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidJSON)
	}
}

func TestScreenshotAlias_DelegatesStraightToObserve(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.screenshot = func(req JSONRPCRequest) JSONRPCResponse {
		return succeed(req, "shot", map[string]any{"from": "observe"})
	}
	resp := hs.h.HandleScreenshotAliasImpl(domcovReq(), json.RawMessage(`{"full_page":true}`))
	if hs.screenshotCalls != 1 {
		t.Fatalf("GetScreenshot called %d times, want 1", hs.screenshotCalls)
	}
	if got := domcovPayload(t, resp)["from"]; got != "observe" {
		t.Errorf("payload = %v, want the observe response passed through verbatim", got)
	}
	if len(hs.queries) != 0 {
		t.Errorf("the alias must not enqueue its own command: %+v", hs.queries)
	}
}

// ---------------------------------------------------------------------------
// interact_browser_tabs.go
// ---------------------------------------------------------------------------

func TestNewTab_DoesNotRequireAnAlreadyTrackedTab(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionNewTabImpl(domcovReq(), json.RawMessage(`{"url":"https://example.com"}`))
	// new_tab is how tracking gets bootstrapped, so requireTabTracking must be skipped.
	want := []string{"pilot", "extension"}
	if strings.Join(hs.guardCalls, ",") != strings.Join(want, ",") {
		t.Errorf("guards = %v, want %v", hs.guardCalls, want)
	}
}

func TestNewTab_ResolvedURLIsDispatchedAndBothURLsRecorded(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.listenPort = 8123
	hs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	hs.h.HandleBrowserActionNewTabImpl(domcovReq(), json.RawMessage(`{"url":"kaboom-insecure://https://bad-cert.test/p"}`))

	q := hs.domcovOnlyQuery()
	if q.Type != "browser_action" || q.Params["action"] != "new_tab" {
		t.Errorf("query = %q/%v", q.Type, q.Params["action"])
	}
	wantURL := "http://127.0.0.1:8123/insecure-proxy?target=https%3A%2F%2Fbad-cert.test%2Fp"
	if q.Params["url"] != wantURL {
		t.Errorf("dispatched url = %v, want %q", q.Params["url"], wantURL)
	}
	ai := hs.domcovOnlyAI()
	if ai.Extra["target_url"] != wantURL {
		t.Errorf("target_url = %v, want the rewritten URL", ai.Extra["target_url"])
	}
	if ai.Extra["requested_url"] != "kaboom-insecure://https://bad-cert.test/p" {
		t.Errorf("requested_url = %v, want the caller's original URL", ai.Extra["requested_url"])
	}
}

func TestNewTab_UnresolvableInsecureURLIsRejectedWithConfigureGuidance(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleBrowserActionNewTabImpl(domcovReq(), json.RawMessage(`{"url":"kaboom-insecure://https://x.test/"}`))
	se := domcovError(t, resp)
	if se.ErrorCode != ErrInvalidParam || se.Param != "url" {
		t.Errorf("got %q/%q, want invalid_param/url", se.ErrorCode, se.Param)
	}
	if !strings.Contains(se.RecoveryPlaybook, "insecure_proxy") {
		t.Errorf("playbook = %q, want the configure() recovery step", se.RecoveryPlaybook)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestNewTab_WithoutURLOmitsTheURLKeyEntirely(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionNewTabImpl(domcovReq(), json.RawMessage(`{}`))
	q := hs.domcovOnlyQuery()
	if _, present := q.Params["url"]; present {
		t.Errorf("url must be omitted when unset, got %v", q.Params["url"])
	}
	if q.Params["action"] != "new_tab" {
		t.Errorf("action = %v, want new_tab", q.Params["action"])
	}
}

func TestNewTab_MalformedJSONIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleBrowserActionNewTabImpl(domcovReq(), json.RawMessage(`{"url":`)))
	if se.ErrorCode != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidJSON)
	}
}

func TestSwitchTab_RequiresATabIDOrIndexAndOffersBothAlternatives(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{}`)))
	if se.ErrorCode != ErrMissingParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrMissingParam)
	}
	if se.Message != "switch_tab requires tab_id or tab_index" {
		t.Errorf("message = %q", se.Message)
	}
	if se.Param != "tab_id" {
		t.Errorf("param = %q, want tab_id", se.Param)
	}
	if !strings.Contains(se.Hint, "tab_index") {
		t.Errorf("hint = %q, want it to mention the tab_index alternative", se.Hint)
	}
}

func TestSwitchTab_NonPositiveTabIDStillNeedsAnIndex(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	// tab_id 0 is Chrome's "no tab" sentinel and must not be treated as a target.
	se := domcovError(t, hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{"tab_id":0}`)))
	if se.Message != "switch_tab requires tab_id or tab_index" {
		t.Errorf("message = %q", se.Message)
	}
}

func TestSwitchTab_NegativeTabIndexIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{"tab_index":-1}`)))
	if se.ErrorCode != ErrInvalidParam || se.Param != "tab_index" {
		t.Errorf("got %q/%q, want invalid_param/tab_index", se.ErrorCode, se.Param)
	}
	if se.Message != "tab_index must be >= 0" {
		t.Errorf("message = %q", se.Message)
	}
}

func TestSwitchTab_IndexZeroIsAValidTarget(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{"tab_index":0}`))
	q := hs.domcovOnlyQuery()
	if q.Params["action"] != "switch_tab" || q.Params["tab_index"] != float64(0) {
		t.Errorf("query params = %v, want a switch_tab to index 0", q.Params)
	}
}

func TestSwitchTab_DoesNotRequireExistingTabTracking(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{"tab_id":7}`))
	// switch_tab IS how tracking is established, so it must not be gated on it.
	want := []string{"pilot", "extension"}
	if strings.Join(hs.guardCalls, ",") != strings.Join(want, ",") {
		t.Errorf("guards = %v, want %v", hs.guardCalls, want)
	}
	if q := hs.domcovOnlyQuery(); q.TabID != 7 {
		t.Errorf("query tab_id = %d, want 7", q.TabID)
	}
}

func TestSwitchTab_SetTrackedFalseSkipsTheServerSideRetarget(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{"tab_id":7,"set_tracked":false}`))
	if hs.captureCalls != 0 {
		t.Errorf("set_tracked=false must not touch the tracked-tab state (%d capture reads)", hs.captureCalls)
	}
}

func TestSwitchTab_DefaultsToRetargetingTrackedTab(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), json.RawMessage(`{"tab_id":7}`))
	// Omitting set_tracked must behave like set_tracked=true (issue #271).
	if hs.captureCalls == 0 {
		t.Error("set_tracked defaults to true, so the tracked-tab retarget must be attempted")
	}
}

func TestActivateTab_QueuesTheActivateBrowserAction(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleActivateTabImpl(domcovReq(), json.RawMessage(`{}`))
	q := hs.domcovOnlyQuery()
	if q.Type != "browser_action" || q.Params["action"] != "activate_tab" {
		t.Errorf("query = %q/%v", q.Type, q.Params["action"])
	}
	if !strings.HasPrefix(q.CorrelationID, "activate_") {
		t.Errorf("correlation_id = %q, want an activate_ prefix", q.CorrelationID)
	}
	if got := domcovSummary(t, resp); got != "Activate tab queued" {
		t.Errorf("summary = %q", got)
	}
	if hs.domcovOnlyAI().Action != "activate_tab" {
		t.Errorf("recorded action = %q", hs.domcovOnlyAI().Action)
	}
}

func TestCloseTab_ForwardsTabIDToTheQueryAndTheActionLog(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionCloseTabImpl(domcovReq(), json.RawMessage(`{"tab_id":19}`))
	q := hs.domcovOnlyQuery()
	if q.TabID != 19 {
		t.Errorf("query tab_id = %d, want 19", q.TabID)
	}
	if q.Params["action"] != "close_tab" || q.Params["tab_id"] != float64(19) {
		t.Errorf("query params = %v", q.Params)
	}
	if got := hs.domcovOnlyAI().Extra["tab_id"]; got != 19 {
		t.Errorf("recorded tab_id = %v, want 19", got)
	}
}

func TestCloseTab_IsGatedOnTabTrackingEvenWithAnExplicitTabID(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockTab = true
	resp := hs.h.HandleBrowserActionCloseTabImpl(domcovReq(), json.RawMessage(`{"tab_id":19}`))
	if se := domcovError(t, resp); !strings.Contains(se.Message, "tab_tracking") {
		t.Errorf("message = %q, want the tab-tracking guard rejection", se.Message)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

// ---------------------------------------------------------------------------
// interact_browser_navigation_impl.go
// ---------------------------------------------------------------------------

func TestNavigate_MissingURLIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleBrowserActionNavigateImpl(domcovReq(), json.RawMessage(`{}`)))
	if se.ErrorCode != ErrMissingParam || se.Param != "url" {
		t.Errorf("got %q/%q, want missing_param/url", se.ErrorCode, se.Param)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestNavigate_UnresolvableInsecureURLIsRejectedBeforeGuards(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleBrowserActionNavigateImpl(domcovReq(), json.RawMessage(`{"url":"kaboom-insecure://https://x.test/"}`))
	se := domcovError(t, resp)
	if se.ErrorCode != ErrInvalidParam || se.Param != "url" {
		t.Errorf("got %q/%q, want invalid_param/url", se.ErrorCode, se.Param)
	}
	if hs.injectCalls != 0 {
		t.Errorf("a rejected navigate must not run CSP injection (%d calls)", hs.injectCalls)
	}
}

func TestNavigate_DispatchesResolvedURLAndStashesAPerfBaseline(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.cap.UpdateTrackedTab(2, "https://example.com/old", "Old")
	hs.cap.AddPerformanceSnapshots([]capture.PerformanceSnapshot{{URL: "/old", Timestamp: "before"}})

	resp := hs.h.HandleBrowserActionNavigateImpl(domcovReq(),
		json.RawMessage(`{"url":"https://example.com/new","tab_id":2}`))

	q := hs.domcovOnlyQuery()
	if q.Type != "browser_action" || q.Params["action"] != "navigate" {
		t.Errorf("query = %q/%v", q.Type, q.Params["action"])
	}
	if q.Params["url"] != "https://example.com/new" {
		t.Errorf("url = %v", q.Params["url"])
	}
	if q.TabID != 2 {
		t.Errorf("tab_id = %d, want 2", q.TabID)
	}
	// The perf baseline must be captured *before* the navigation is queued so
	// perf_diff has a pre-navigation reference.
	snap, ok := hs.cap.GetAndDeleteBeforeSnapshot(q.CorrelationID)
	if !ok || snap.Timestamp != "before" {
		t.Errorf("before-snapshot for %q = %+v/%v, want the pre-navigation snapshot", q.CorrelationID, snap, ok)
	}
	if hs.injectCalls != 1 {
		t.Errorf("CSP blocked-action injection ran %d times, want 1", hs.injectCalls)
	}
	if got := domcovSummary(t, resp); got != "Navigate queued" {
		t.Errorf("summary = %q", got)
	}
}

func TestNavigate_ContentEnrichmentOnlyRunsWhenRequested(t *testing.T) {
	t.Parallel()
	t.Run("omitted", func(t *testing.T) {
		t.Parallel()
		hs := domcovNew(t)
		hs.h.HandleBrowserActionNavigateImpl(domcovReq(), json.RawMessage(`{"url":"https://a.test/"}`))
		if hs.enrichCalls != 0 {
			t.Errorf("enrichment ran %d times without include_content", hs.enrichCalls)
		}
	})
	t.Run("requested", func(t *testing.T) {
		t.Parallel()
		hs := domcovNew(t)
		hs.enrichNavigate = func(resp JSONRPCResponse) JSONRPCResponse {
			return mutateToolResult(resp, func(r *MCPToolResult) {
				r.Content = append(r.Content, MCPContentBlock{Type: "text", Text: "PAGE BODY"})
			})
		}
		resp := hs.h.HandleBrowserActionNavigateImpl(domcovReq(),
			json.RawMessage(`{"url":"https://a.test/","include_content":true,"tab_id":5}`))

		if hs.enrichCalls != 1 {
			t.Fatalf("enrichment ran %d times, want 1", hs.enrichCalls)
		}
		if len(hs.enrichTabIDs) != 1 || hs.enrichTabIDs[0] != 5 {
			t.Errorf("enrichment tab ids = %v, want [5]", hs.enrichTabIDs)
		}
		if !strings.Contains(string(resp.Result), "PAGE BODY") {
			t.Errorf("enriched content missing from response: %s", resp.Result)
		}
	})
}

func TestRefresh_SendsOnlyTheActionAndKeepsTabIDOnTheEnvelope(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionRefreshImpl(domcovReq(), json.RawMessage(`{"tab_id":4,"noise":"x"}`))
	q := hs.domcovOnlyQuery()
	// refresh rebuilds its params rather than forwarding args, so caller extras
	// must not leak into the extension payload.
	if len(q.Params) != 1 || q.Params["action"] != "refresh" {
		t.Errorf("query params = %v, want exactly {action: refresh}", q.Params)
	}
	if q.TabID != 4 {
		t.Errorf("tab_id = %d, want 4", q.TabID)
	}
}

func TestRefresh_IsGatedOnTabTracking(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleBrowserActionRefreshImpl(domcovReq(), json.RawMessage(`{}`))
	want := []string{"pilot", "extension", "tab_tracking"}
	if strings.Join(hs.guardCalls, ",") != strings.Join(want, ",") {
		t.Errorf("guards = %v, want %v", hs.guardCalls, want)
	}
}

func TestHistoryNavigation_BackAndForwardUseDistinctActionsAndPrefixes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, action, prefix, summary string
		call                          func(*domcovHarness) JSONRPCResponse
	}{
		{"back", "back", "back_", "Back queued", func(hs *domcovHarness) JSONRPCResponse {
			return hs.h.HandleBrowserActionBackImpl(domcovReq(), json.RawMessage(`{}`))
		}},
		{"forward", "forward", "forward_", "Forward queued", func(hs *domcovHarness) JSONRPCResponse {
			return hs.h.HandleBrowserActionForwardImpl(domcovReq(), json.RawMessage(`{}`))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			resp := tc.call(hs)
			q := hs.domcovOnlyQuery()
			if q.Type != "browser_action" || q.Params["action"] != tc.action {
				t.Errorf("query = %q/%v, want browser_action/%s", q.Type, q.Params["action"], tc.action)
			}
			if !strings.HasPrefix(q.CorrelationID, tc.prefix) {
				t.Errorf("correlation_id = %q, want prefix %q", q.CorrelationID, tc.prefix)
			}
			if got := domcovSummary(t, resp); got != tc.summary {
				t.Errorf("summary = %q, want %q", got, tc.summary)
			}
			if hs.domcovOnlyAI().Action != tc.action {
				t.Errorf("recorded action = %q, want %q", hs.domcovOnlyAI().Action, tc.action)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// interact_browser_script_impl.go
// ---------------------------------------------------------------------------

func TestHighlight_MissingSelectorIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleHighlightImpl(domcovReq(), json.RawMessage(`{"duration_ms":500}`)))
	if se.ErrorCode != ErrMissingParam || se.Param != "selector" {
		t.Errorf("got %q/%q, want missing_param/selector", se.ErrorCode, se.Param)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestHighlight_ForwardsTheCallerArgsVerbatimAsQueryParams(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleHighlightImpl(domcovReq(), json.RawMessage(`{"selector":".row","duration_ms":750,"tab_id":6}`))
	q := hs.domcovOnlyQuery()
	if q.Type != "highlight" {
		t.Errorf("query type = %q, want highlight", q.Type)
	}
	if q.Params["selector"] != ".row" || q.Params["duration_ms"] != float64(750) {
		t.Errorf("query params = %v", q.Params)
	}
	if q.TabID != 6 {
		t.Errorf("tab_id = %d, want 6", q.TabID)
	}
	if got := hs.domcovOnlyAI().Extra["selector"]; got != ".row" {
		t.Errorf("recorded selector = %v", got)
	}
}

func TestExecuteJS_MissingScriptIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleExecuteJSImpl(domcovReq(), json.RawMessage(`{"world":"main"}`)))
	if se.ErrorCode != ErrMissingParam || se.Param != "script" {
		t.Errorf("got %q/%q, want missing_param/script", se.ErrorCode, se.Param)
	}
	if len(hs.cspWorlds) != 0 {
		t.Errorf("CSP must not be consulted for an invalid request: %v", hs.cspWorlds)
	}
}

func TestExecuteJS_UnknownWorldIsRejectedAndListsTheValidWorlds(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleExecuteJSImpl(domcovReq(), json.RawMessage(`{"script":"1","world":"page"}`)))
	if se.ErrorCode != ErrInvalidParam || se.Param != "world" {
		t.Errorf("got %q/%q, want invalid_param/world", se.ErrorCode, se.Param)
	}
	if se.Message != "Invalid 'world' value: page" {
		t.Errorf("message = %q", se.Message)
	}
	for _, w := range []string{"auto", "main", "isolated"} {
		if !strings.Contains(se.RecoveryPlaybook, "'"+w+"'") {
			t.Errorf("playbook %q must list world %q", se.RecoveryPlaybook, w)
		}
	}
}

func TestExecuteJS_WorldDefaultsToAutoAndIsCSPChecked(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleExecuteJSImpl(domcovReq(), json.RawMessage(`{"script":"return 1"}`))
	if len(hs.cspWorlds) != 1 || hs.cspWorlds[0] != "auto" {
		t.Errorf("CSP worlds = %v, want [auto]", hs.cspWorlds)
	}
	if q := hs.domcovOnlyQuery(); q.Type != "execute" {
		t.Errorf("query type = %q, want execute", q.Type)
	}
}

func TestExecuteJS_CSPBlockPreventsDispatch(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockCSP = true
	resp := hs.h.HandleExecuteJSImpl(domcovReq(), json.RawMessage(`{"script":"1","world":"main"}`))
	if se := domcovError(t, resp); !strings.Contains(se.Message, "csp blocked world=main") {
		t.Errorf("message = %q, want the CSP rejection for world=main", se.Message)
	}
	if len(hs.queries) != 0 {
		t.Errorf("a CSP-blocked script must not be enqueued: %+v", hs.queries)
	}
	if len(hs.aiActions) != 0 {
		t.Errorf("a CSP-blocked script must not be recorded: %+v", hs.aiActions)
	}
}

func TestExecuteJS_RecordedScriptPreviewIsTruncatedTo100Chars(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	script := strings.Repeat("a", 250)
	hs.h.HandleExecuteJSImpl(domcovReq(), json.RawMessage(`{"script":"`+script+`"}`))

	preview, _ := hs.domcovOnlyAI().Extra["script_preview"].(string)
	// The action log is user-visible; an untruncated preview would dump whole
	// scripts into it.
	if preview != strings.Repeat("a", 100)+"..." {
		t.Errorf("script_preview = %q (len %d), want 100 chars plus an ellipsis", preview, len(preview))
	}
	// The full script still has to reach the extension.
	if got := hs.domcovOnlyQuery().Params["script"]; got != script {
		t.Errorf("dispatched script was truncated: %v", got)
	}
}

// ---------------------------------------------------------------------------
// interact_clipboard.go
// ---------------------------------------------------------------------------

func TestClipboardWrite_MissingTextIsRejected(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	se := domcovError(t, hs.h.HandleClipboardWrite(domcovReq(), json.RawMessage(`{}`)))
	if se.ErrorCode != ErrMissingParam || se.Param != "text" {
		t.Errorf("got %q/%q, want missing_param/text", se.ErrorCode, se.Param)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
	if len(hs.aiActions) != 0 {
		t.Errorf("must not record an action: %+v", hs.aiActions)
	}
}

func TestClipboardWrite_TextIsEmbeddedAsAJSONLiteralNotRawlyConcatenated(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	// A quote/backslash/newline payload would break out of the generated script
	// if it were concatenated raw — this pins the JSON-literal encoding.
	hs.h.HandleClipboardWrite(domcovReq(), json.RawMessage(`{"text":"a\"b\\c\nd"}`))

	script, _ := hs.domcovOnlyQuery().Params["script"].(string)
	if !strings.Contains(script, `writeText("a\"b\\c\nd")`) {
		t.Fatalf("script must embed the text as a JSON literal, got:\n%s", script)
	}
	if strings.Contains(script, "writeText(a") {
		t.Errorf("text was concatenated unquoted:\n%s", script)
	}
}

func TestClipboardWrite_RecordsOnlyTheTextLengthNotTheContent(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleClipboardWrite(domcovReq(), json.RawMessage(`{"text":"secret-token"}`))

	ai := hs.domcovOnlyAI()
	if ai.Action != "clipboard_write" {
		t.Errorf("recorded action = %q", ai.Action)
	}
	if ai.Extra["text_length"] != len("secret-token") {
		t.Errorf("text_length = %v, want %d", ai.Extra["text_length"], len("secret-token"))
	}
	// The clipboard content itself must not be copied into the action log.
	for k, v := range ai.Extra {
		if s, ok := v.(string); ok && strings.Contains(s, "secret-token") {
			t.Errorf("action log leaks clipboard content in %q: %v", k, v)
		}
	}
}

func TestClipboardWrite_RunsInTheMainWorldViaTheExecuteQuery(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleClipboardWrite(domcovReq(), json.RawMessage(`{"text":"x"}`))
	q := hs.domcovOnlyQuery()
	if q.Type != "execute" {
		t.Errorf("query type = %q, want execute", q.Type)
	}
	// navigator.clipboard is unavailable in the isolated world.
	if q.Params["world"] != "main" {
		t.Errorf("world = %v, want main", q.Params["world"])
	}
	if q.Params["reason"] != "clipboard_write" {
		t.Errorf("reason = %v, want clipboard_write", q.Params["reason"])
	}
}

func TestClipboardRead_QueuesAReadTextScriptAndRecordsTheAction(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleClipboardRead(domcovReq(), json.RawMessage(`{}`))

	q := hs.domcovOnlyQuery()
	script, _ := q.Params["script"].(string)
	if !strings.Contains(script, "navigator.clipboard.readText()") {
		t.Errorf("script must call readText, got:\n%s", script)
	}
	if !strings.Contains(script, "clipboard_read_failed") {
		t.Errorf("script must return a structured failure marker, got:\n%s", script)
	}
	if q.Params["world"] != "main" {
		t.Errorf("world = %v, want main", q.Params["world"])
	}
	if hs.domcovOnlyAI().Action != "clipboard_read" {
		t.Errorf("recorded action = %q", hs.domcovOnlyAI().Action)
	}
	if got := domcovSummary(t, resp); got != "Clipboard read queued" {
		t.Errorf("summary = %q", got)
	}
}

func TestClipboardRead_GuardRejectionSkipsActionRecording(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockPilot = true
	hs.h.HandleClipboardRead(domcovReq(), json.RawMessage(`{}`))
	if len(hs.aiActions) != 0 {
		t.Errorf("a blocked clipboard read must not be recorded: %+v", hs.aiActions)
	}
}

// ---------------------------------------------------------------------------
// interact_draw.go
// ---------------------------------------------------------------------------

func TestDrawModeStart_QueuesTheStartActionAndReturnsACorrelationID(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleDrawModeStart(domcovReq(), json.RawMessage(`{"tab_id":12}`))

	q := hs.domcovOnlyQuery()
	if q.Type != "draw_mode" {
		t.Errorf("query type = %q, want draw_mode", q.Type)
	}
	if len(q.Params) != 1 || q.Params["action"] != "start" {
		t.Errorf("query params = %v, want exactly {action: start}", q.Params)
	}
	if q.TabID != 12 {
		t.Errorf("tab_id = %d, want 12", q.TabID)
	}
	payload := domcovPayload(t, resp)
	if payload["status"] != "queued" {
		t.Errorf("status = %v, want queued", payload["status"])
	}
	if payload["correlation_id"] != q.CorrelationID {
		t.Errorf("correlation_id = %v, want the enqueued %q", payload["correlation_id"], q.CorrelationID)
	}
	if !strings.HasPrefix(q.CorrelationID, "draw_") {
		t.Errorf("correlation_id = %q, want a draw_ prefix", q.CorrelationID)
	}
	if got := domcovSummary(t, resp); got != "Draw mode activated" {
		t.Errorf("summary = %q", got)
	}
	if hs.drawStarted != 1 {
		t.Errorf("MarkDrawStarted called %d times, want 1", hs.drawStarted)
	}
	if hs.domcovOnlyAI().Action != "draw_mode_start" {
		t.Errorf("recorded action = %q", hs.domcovOnlyAI().Action)
	}
	// Draw mode is fire-and-forget; it must not block on MaybeWaitForCommand.
	if len(hs.waitCalls) != 0 {
		t.Errorf("draw_mode_start must not wait for a command result: %v", hs.waitCalls)
	}
}

func TestDrawModeStart_AnnotSessionIsForwardedWhenPresent(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleDrawModeStart(domcovReq(), json.RawMessage(`{"annot_session":"sess-7"}`))
	if got := hs.domcovOnlyQuery().Params["annot_session"]; got != "sess-7" {
		t.Errorf("annot_session = %v, want sess-7", got)
	}
}

func TestDrawModeStart_BlockedEnqueueLeavesDrawStateUnarmed(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockEnqueue = true
	resp := hs.h.HandleDrawModeStart(domcovReq(), json.RawMessage(`{}`))

	if se := domcovError(t, resp); se.ErrorCode != ErrQueueFull {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrQueueFull)
	}
	// MarkDrawStarted sets WaitForSession's timestamp baseline. Arming it for a
	// command that never queued would make analyze(annotations) wait forever.
	if hs.drawStarted != 0 {
		t.Errorf("MarkDrawStarted ran %d times for a command that never queued", hs.drawStarted)
	}
	if len(hs.aiActions) != 0 {
		t.Errorf("must not record an action: %+v", hs.aiActions)
	}
}

func TestDrawModeStart_GuardRejectionHappensBeforeEnqueue(t *testing.T) {
	t.Parallel()
	for name, block := range map[string]func(*domcovHarness){
		"pilot off":     func(hs *domcovHarness) { hs.blockPilot = true },
		"no extension":  func(hs *domcovHarness) { hs.blockExt = true },
		"no tab traced": func(hs *domcovHarness) { hs.blockTab = true },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			block(hs)
			resp := hs.h.HandleDrawModeStart(domcovReq(), json.RawMessage(`{}`))
			if se := domcovError(t, resp); se.ErrorCode != ErrCodePilotDisabled {
				t.Errorf("error_code = %q, want the guard rejection", se.ErrorCode)
			}
			if len(hs.queries) != 0 || hs.drawStarted != 0 {
				t.Errorf("blocked draw start still had side effects: queries=%+v drawStarted=%d", hs.queries, hs.drawStarted)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// interact_explore.go
// ---------------------------------------------------------------------------

func TestExplorePage_OnlyHTTPAndHTTPSURLsAreAccepted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, url, wantMsg string
	}{
		// #341: file:// and javascript: must never reach the extension.
		{"file scheme", "file:///etc/passwd", "Only http and https URLs are allowed, got: file"},
		{"javascript scheme", "javascript:alert(1)", "Only http and https URLs are allowed, got: javascript"},
		{"ftp scheme", "ftp://host/f", "Only http and https URLs are allowed, got: ftp"},
		{"scheme-less", "example.com/path", "Invalid URL: example.com/path"},
		{"control chars", "ht\ntp://x", "Invalid URL: ht\ntp://x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			args, err := json.Marshal(map[string]any{"url": tc.url})
			if err != nil {
				t.Fatalf("marshal args: %v", err)
			}
			resp := hs.h.HandleExplorePage(domcovReq(), args)
			se := domcovError(t, resp)
			if se.ErrorCode != ErrInvalidParam || se.Param != "url" {
				t.Errorf("got %q/%q, want invalid_param/url", se.ErrorCode, se.Param)
			}
			if se.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", se.Message, tc.wantMsg)
			}
			if len(hs.queries) != 0 {
				t.Errorf("must not enqueue: %+v", hs.queries)
			}
		})
	}
}

func TestExplorePage_EmptyArgsAreAllowedAndQueueTheCompoundQuery(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	resp := hs.h.HandleExplorePage(domcovReq(), nil)
	q := hs.domcovOnlyQuery()
	if q.Type != "explore_page" {
		t.Errorf("query type = %q, want explore_page", q.Type)
	}
	if got := domcovSummary(t, resp); got != "Explore page queued" {
		t.Errorf("summary = %q", got)
	}
	// A queued (not-yet-complete) response must not be post-processed.
	if hs.screenshotCalls != 0 {
		t.Errorf("a queued explore must not capture a screenshot (%d calls)", hs.screenshotCalls)
	}
}

func TestExplorePage_ValidHTTPSURLIsForwardedAndRecorded(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleExplorePage(domcovReq(), json.RawMessage(`{"url":"https://example.com/x","tab_id":3,"visible_only":true}`))
	q := hs.domcovOnlyQuery()
	if q.Params["url"] != "https://example.com/x" || q.Params["visible_only"] != true {
		t.Errorf("query params = %v", q.Params)
	}
	if q.TabID != 3 {
		t.Errorf("tab_id = %d, want 3", q.TabID)
	}
	if ai := hs.domcovOnlyAI(); ai.Action != "explore_page" || ai.URL != "https://example.com/x" {
		t.Errorf("recorded AI action = %+v", ai)
	}
}

// domcovExploreBody builds a completed explore_page response body.
func domcovExploreBody(t *testing.T, req JSONRPCRequest, data map[string]any) JSONRPCResponse {
	t.Helper()
	return succeed(req, "explore complete", data)
}

func TestExplorePage_CompletedResultGainsMenusAndAScreenshot(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.waitResponse = func(req JSONRPCRequest, _ string, _ string) JSONRPCResponse {
		return domcovExploreBody(t, req, map[string]any{
			"status":            "complete",
			"interactive_count": 3,
			"interactive_elements": []any{
				map[string]any{"index": 0, "label": "Home", "tag": "a", "href": "/", "visible": true,
					"landmark_tag": "nav", "bbox": map[string]any{"x": 0, "y": 0, "width": 60, "height": 20}},
				map[string]any{"index": 1, "label": "Docs", "tag": "a", "href": "/d", "visible": true,
					"landmark_tag": "nav", "bbox": map[string]any{"x": 70, "y": 0, "width": 60, "height": 20}},
				map[string]any{"index": 2, "label": "Buy", "tag": "button", "visible": true,
					"bbox": map[string]any{"x": 400, "y": 400, "width": 80, "height": 30}},
			},
		})
	}
	hs.screenshot = func(req JSONRPCRequest) JSONRPCResponse {
		result := MCPToolResult{Content: []MCPContentBlock{{Type: "image", Data: "AAAA", MimeType: "image/png"}}}
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal screenshot: %v", err)
		}
		return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: raw}
	}

	resp := hs.h.HandleExplorePage(domcovReq(), json.RawMessage(`{}`))

	payload := domcovPayload(t, resp)
	if _, ok := payload["site_menus"]; !ok {
		t.Fatalf("completed explore must gain site_menus, got %v", payload)
	}
	// The two <nav> links are claimed by a menu group and must be removed from
	// interactive_elements so agents do not see them twice.
	remaining, ok := payload["interactive_elements"].([]any)
	if !ok {
		t.Fatalf("interactive_elements missing: %v", payload)
	}
	if len(remaining) != 1 {
		t.Fatalf("interactive_elements = %d entries, want 1 unclaimed element", len(remaining))
	}
	if got := remaining[0].(map[string]any)["label"]; got != "Buy" {
		t.Errorf("remaining element = %v, want the non-menu Buy button", got)
	}
	if payload["interactive_count"] != float64(1) {
		t.Errorf("interactive_count = %v, want 1 after 2 elements were claimed", payload["interactive_count"])
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	last := result.Content[len(result.Content)-1]
	if last.Type != "image" || last.Data != "AAAA" {
		t.Errorf("last content block = %+v, want the appended screenshot", last)
	}
}

func TestEnrichExploreWithMenus_LeavesUnusableBodiesUntouched(t *testing.T) {
	t.Parallel()
	req := domcovReq()
	cases := map[string]JSONRPCResponse{
		"no elements key": succeed(req, "s", map[string]any{"title": "T"}),
		"empty elements":  succeed(req, "s", map[string]any{"interactive_elements": []any{}}),
	}
	for name, resp := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := enrichExploreWithMenus(resp)
			if string(got.Result) != string(resp.Result) {
				t.Errorf("response changed:\n got %s\nwant %s", got.Result, resp.Result)
			}
			// Without elements there is nothing to group, so no empty site_menus
			// section should be invented.
			if strings.Contains(string(got.Result), "site_menus") {
				t.Errorf("site_menus must not be added: %s", got.Result)
			}
		})
	}
}

func TestEnrichExploreWithMenus_NonJSONContentIsPassedThrough(t *testing.T) {
	t.Parallel()
	req := domcovReq()
	plain := MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: "no json here"}}}
	raw, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp := JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: raw}
	if got := enrichExploreWithMenus(resp); string(got.Result) != string(resp.Result) {
		t.Errorf("response changed: %s", got.Result)
	}

	broken := MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: "summary\n{not json"}}}
	brokenRaw, err := json.Marshal(broken)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	brokenResp := JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: brokenRaw}
	if got := enrichExploreWithMenus(brokenResp); string(got.Result) != string(brokenResp.Result) {
		t.Errorf("malformed JSON body must pass through unchanged: %s", got.Result)
	}
}

func TestEnrichExploreWithMenus_UnclaimedElementsKeepTheOriginalCount(t *testing.T) {
	t.Parallel()
	// Two far-apart elements with no landmark form no menu group, so nothing is
	// filtered and interactive_count must be left alone.
	resp := succeed(domcovReq(), "s", map[string]any{
		"interactive_count": 2,
		"interactive_elements": []any{
			map[string]any{"index": 0, "label": "A", "tag": "button", "visible": true,
				"bbox": map[string]any{"x": 0, "y": 0, "width": 10, "height": 10}},
			map[string]any{"index": 1, "label": "B", "tag": "button", "visible": true,
				"bbox": map[string]any{"x": 900, "y": 700, "width": 10, "height": 10}},
		},
	})
	payload := domcovPayload(t, enrichExploreWithMenus(resp))
	if payload["interactive_count"] != float64(2) {
		t.Errorf("interactive_count = %v, want 2", payload["interactive_count"])
	}
	if got := payload["interactive_elements"].([]any); len(got) != 2 {
		t.Errorf("interactive_elements = %d, want both kept", len(got))
	}
	menus, ok := payload["site_menus"].(map[string]any)
	if !ok {
		t.Fatalf("site_menus missing: %v", payload)
	}
	if ungrouped, _ := menus["ungrouped"].([]any); len(ungrouped) != 2 {
		t.Errorf("ungrouped = %v, want both elements reported as ungrouped", menus["ungrouped"])
	}
}

// ---------------------------------------------------------------------------
// interact_content.go
// ---------------------------------------------------------------------------

func TestContentExtraction_TimeoutIsDefaultedAndClamped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args string
		want float64
	}{
		{"omitted", `{}`, 10000},
		{"zero", `{"timeout_ms":0}`, 10000},
		{"negative", `{"timeout_ms":-5}`, 10000},
		{"in range", `{"timeout_ms":2500}`, 2500},
		{"at cap", `{"timeout_ms":30000}`, 30000},
		{"over cap", `{"timeout_ms":120000}`, 30000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			hs.h.HandleContentExtraction(domcovReq(), json.RawMessage(tc.args), "get_readable", "readable")
			if got := hs.domcovOnlyQuery().Params["timeout_ms"]; got != tc.want {
				t.Errorf("timeout_ms = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestContentExtraction_OnlyTimeoutReachesTheExtension(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.HandleContentExtraction(domcovReq(), json.RawMessage(`{"tab_id":9,"selector":"#main"}`), "page_summary", "summary")
	q := hs.domcovOnlyQuery()
	// The params are rebuilt, not forwarded — caller extras must not leak.
	if len(q.Params) != 1 {
		t.Errorf("query params = %v, want only timeout_ms", q.Params)
	}
	if q.TabID != 9 {
		t.Errorf("tab_id = %d, want 9 on the query envelope", q.TabID)
	}
	if q.Type != "page_summary" {
		t.Errorf("query type = %q, want page_summary", q.Type)
	}
	if !strings.HasPrefix(q.CorrelationID, "summary_") {
		t.Errorf("correlation_id = %q, want a summary_ prefix", q.CorrelationID)
	}
}

func TestContentExtraction_MalformedArgsFallBackToDefaultsInsteadOfFailing(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	// Content extraction parses leniently: garbage args must still run with defaults.
	hs.h.HandleGetReadable(domcovReq(), json.RawMessage(`not-json`))
	q := hs.domcovOnlyQuery()
	if q.Params["timeout_ms"] != float64(10000) {
		t.Errorf("timeout_ms = %v, want the 10000ms default", q.Params["timeout_ms"])
	}
	if q.TabID != 0 {
		t.Errorf("tab_id = %d, want 0", q.TabID)
	}
}

func TestContentExtraction_ReadableAndMarkdownUseDistinctQueryTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, queryType, prefix, summary string
		call                             func(*domcovHarness) JSONRPCResponse
	}{
		{"get_readable", "get_readable", "readable_", "get_readable queued", func(hs *domcovHarness) JSONRPCResponse {
			return hs.h.HandleGetReadable(domcovReq(), json.RawMessage(`{}`))
		}},
		{"get_markdown", "get_markdown", "markdown_", "get_markdown queued", func(hs *domcovHarness) JSONRPCResponse {
			return hs.h.HandleGetMarkdown(domcovReq(), json.RawMessage(`{}`))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			resp := tc.call(hs)
			q := hs.domcovOnlyQuery()
			if q.Type != tc.queryType {
				t.Errorf("query type = %q, want %q", q.Type, tc.queryType)
			}
			if !strings.HasPrefix(q.CorrelationID, tc.prefix) {
				t.Errorf("correlation_id = %q, want prefix %q", q.CorrelationID, tc.prefix)
			}
			if got := domcovSummary(t, resp); got != tc.summary {
				t.Errorf("summary = %q, want %q", got, tc.summary)
			}
		})
	}
}

func TestContentExtraction_IsGatedOnTabTracking(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.blockTab = true
	resp := hs.h.HandleGetMarkdown(domcovReq(), json.RawMessage(`{}`))
	if se := domcovError(t, resp); !strings.Contains(se.Message, "tab_tracking") {
		t.Errorf("message = %q, want the tab-tracking guard rejection", se.Message)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestNavigatePageSummaryWait_ExceedsTheExtensionSideBudget(t *testing.T) {
	t.Parallel()
	// The extension-side page_summary query uses a 4s timeout; the server wait must
	// be strictly longer or every summary would time out server-side first.
	if NavigatePageSummaryWait <= 4*time.Second {
		t.Errorf("NavigatePageSummaryWait = %v, want > 4s", NavigatePageSummaryWait)
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: malformed args must be rejected by every handler that parses strictly
// ---------------------------------------------------------------------------

func TestHandlers_MalformedJSONIsRejectedBeforeAnySideEffect(t *testing.T) {
	t.Parallel()
	bad := json.RawMessage(`{"tab_id":`)
	cases := map[string]func(*domcovHarness) JSONRPCResponse{
		"navigate": func(hs *domcovHarness) JSONRPCResponse { return hs.h.HandleBrowserActionNavigateImpl(domcovReq(), bad) },
		"refresh":  func(hs *domcovHarness) JSONRPCResponse { return hs.h.HandleBrowserActionRefreshImpl(domcovReq(), bad) },
		"switch_tab": func(hs *domcovHarness) JSONRPCResponse {
			return hs.h.HandleBrowserActionSwitchTabImpl(domcovReq(), bad)
		},
		"close_tab":  func(hs *domcovHarness) JSONRPCResponse { return hs.h.HandleBrowserActionCloseTabImpl(domcovReq(), bad) },
		"highlight":  func(hs *domcovHarness) JSONRPCResponse { return hs.h.HandleHighlightImpl(domcovReq(), bad) },
		"execute_js": func(hs *domcovHarness) JSONRPCResponse { return hs.h.HandleExecuteJSImpl(domcovReq(), bad) },
		"clipboard_write": func(hs *domcovHarness) JSONRPCResponse {
			return hs.h.HandleClipboardWrite(domcovReq(), bad)
		},
		"explore_page": func(hs *domcovHarness) JSONRPCResponse { return hs.h.HandleExplorePage(domcovReq(), bad) },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			se := domcovError(t, call(hs))
			if se.ErrorCode != ErrInvalidJSON {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, ErrInvalidJSON)
			}
			if len(hs.queries) != 0 || len(hs.aiActions) != 0 || len(hs.guardCalls) != 0 {
				t.Errorf("side effects on malformed args: queries=%+v actions=%+v guards=%v",
					hs.queries, hs.aiActions, hs.guardCalls)
			}
		})
	}
}

func TestHandleDOMPrimitive_SelectorAndWaitForValidationBlockDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, action, args, wantCode, wantMsg string
	}{
		{
			"click with no way to identify an element", "click", `{}`,
			ErrMissingParam, "Required parameter 'selector', 'element_id', or 'index' is missing",
		},
		{
			"wait_for with no condition", "wait_for", `{}`,
			ErrMissingParam, "wait_for requires at least one condition: selector, text, or url_contains",
		},
		{
			"wait_for with two conditions", "wait_for", `{"selector":"#a","text":"x"}`,
			ErrInvalidParam, "wait_for conditions are mutually exclusive: use only one of selector, text, or url_contains",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			se := domcovError(t, hs.h.HandleDOMPrimitive(domcovReq(), json.RawMessage(tc.args), tc.action))
			if se.ErrorCode != tc.wantCode {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, tc.wantCode)
			}
			if se.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", se.Message, tc.wantMsg)
			}
			if len(hs.queries) != 0 {
				t.Errorf("must not enqueue: %+v", hs.queries)
			}
		})
	}
}

func TestUpload_EachGuardBlocksBeforeTheFileIsTouched(t *testing.T) {
	t.Parallel()
	for name, block := range map[string]func(*domcovHarness){
		"pilot off":         func(hs *domcovHarness) { hs.blockPilot = true },
		"extension offline": func(hs *domcovHarness) { hs.blockExt = true },
		"tab not tracked":   func(hs *domcovHarness) { hs.blockTab = true },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hs := domcovNew(t)
			block(hs)
			path := domcovTempFile(t, "a.txt", 1)
			resp := hs.up.HandleUpload(domcovReq(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`"}`))
			if se := domcovError(t, resp); se.ErrorCode != ErrCodePilotDisabled {
				t.Errorf("error_code = %q, want the guard rejection", se.ErrorCode)
			}
			if len(hs.queries) != 0 {
				t.Errorf("must not enqueue: %+v", hs.queries)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Remaining edge branches
// ---------------------------------------------------------------------------

func TestResolveNavigateURL_InsecureModeWithoutACaptureStoreIsAnError(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.deps.Capture = func() *capture.Store { return nil }
	_, err := hs.h.ResolveNavigateURLImpl("kaboom-insecure://https://x.test/")
	if err == nil {
		t.Fatal("an uninitialised capture store must fail rather than nil-deref")
	}
	if !strings.Contains(err.Error(), "capture not initialized") {
		t.Errorf("error = %q, want a capture-not-initialized message", err)
	}
}

func TestResolveNavigateURL_UnparseableInsecureTargetIsReported(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	_, err := hs.h.ResolveNavigateURLImpl("kaboom-insecure://http://[::1/x")
	if err == nil {
		t.Fatal("an unparseable target must be rejected")
	}
	if !strings.Contains(err.Error(), "invalid kaboom-insecure target URL") {
		t.Errorf("error = %q, want the invalid-target message", err)
	}
}

func TestQueueBrowserAction_SkipTabGuardDropsOnlyTheTabTrackingCheck(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	hs.h.queueBrowserAction(domcovReq(), json.RawMessage(`{}`), browserActionOpts{
		action:         "probe",
		correlationPfx: "probe",
		skipTabGuard:   true,
		queuedMsg:      "Probe queued",
	})
	want := []string{"pilot", "extension"}
	if strings.Join(hs.guardCalls, ",") != strings.Join(want, ",") {
		t.Errorf("guards = %v, want %v", hs.guardCalls, want)
	}
	q := hs.domcovOnlyQuery()
	// With no explicit params the helper synthesises {"action": <action>}.
	if len(q.Params) != 1 || q.Params["action"] != "probe" {
		t.Errorf("query params = %v, want {action: probe}", q.Params)
	}
	// recordAction defaults to the action name when not overridden.
	if hs.domcovOnlyAI().Action != "probe" {
		t.Errorf("recorded action = %q, want probe", hs.domcovOnlyAI().Action)
	}
}

func TestEnrichExploreWithMenus_NonTextLeadingBlockIsLeftAlone(t *testing.T) {
	t.Parallel()
	result := MCPToolResult{Content: []MCPContentBlock{{Type: "image", Data: "AAAA"}}}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp := JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: float64(1), Result: raw}
	if got := enrichExploreWithMenus(resp); string(got.Result) != string(resp.Result) {
		t.Errorf("an image-first response must pass through unchanged: %s", got.Result)
	}
}

func TestEnrichExploreWithMenus_NonObjectElementEntriesAreKeptNotDropped(t *testing.T) {
	t.Parallel()
	// A malformed entry must survive the menu filter rather than being silently
	// discarded along with the claimed menu items.
	resp := succeed(domcovReq(), "s", map[string]any{
		"interactive_count": 3,
		"interactive_elements": []any{
			"junk-entry",
			map[string]any{"index": 1, "label": "Home", "tag": "a", "visible": true, "landmark_tag": "nav",
				"bbox": map[string]any{"x": 0, "y": 0, "width": 50, "height": 20}},
			map[string]any{"index": 2, "label": "Docs", "tag": "a", "visible": true, "landmark_tag": "nav",
				"bbox": map[string]any{"x": 60, "y": 0, "width": 50, "height": 20}},
		},
	})
	payload := domcovPayload(t, enrichExploreWithMenus(resp))
	remaining, ok := payload["interactive_elements"].([]any)
	if !ok {
		t.Fatalf("interactive_elements missing: %v", payload)
	}
	if len(remaining) != 1 || remaining[0] != "junk-entry" {
		t.Errorf("interactive_elements = %v, want only the unparseable entry retained", remaining)
	}
}

func TestUpload_MissingFileIsReportedAfterGuardsPass(t *testing.T) {
	t.Parallel()
	hs := domcovNew(t)
	missing := filepath.Join(t.TempDir(), "gone.mp4")
	resp := hs.up.HandleUpload(domcovReq(), json.RawMessage(`{"selector":"#f","file_path":"`+missing+`"}`))

	se := domcovError(t, resp)
	if se.ErrorCode != ErrInvalidParam || se.Param != "file_path" {
		t.Errorf("got %q/%q, want invalid_param/file_path", se.ErrorCode, se.Param)
	}
	if !strings.Contains(se.Message, "File not found: "+missing) {
		t.Errorf("message = %q, want it to name the missing path", se.Message)
	}
	if len(hs.guardCalls) != 3 {
		t.Errorf("guards ran %v, want all three to have passed first", hs.guardCalls)
	}
	if len(hs.queries) != 0 {
		t.Errorf("must not enqueue: %+v", hs.queries)
	}
}

func TestUpload_StatErrorsMapToDistinctCodesAndRemedies(t *testing.T) {
	t.Parallel()
	// Driven with synthetic errors: reproducing EACCES via file modes is
	// unreliable (a root CI user bypasses permission bits entirely).
	cases := []struct {
		name      string
		err       error
		wantCode  string
		wantMsg   string
		wantRetry string
		wantParam string
	}{
		{
			"permission denied", os.ErrPermission, ErrPathNotAllowed,
			"Permission denied reading file: /p/f.txt. Check file permissions.",
			"Fix file permissions with: chmod +r /p/f.txt", "file_path",
		},
		{
			"other stat failure", errors.New("i/o error"), ErrInternal,
			"Failed to access file: i/o error",
			"Check the file path and permissions", "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			se := domcovError(t, uploadFileStatError(domcovReq(), "/p/f.txt", tc.err))
			if se.ErrorCode != tc.wantCode {
				t.Errorf("error_code = %q, want %q", se.ErrorCode, tc.wantCode)
			}
			if se.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", se.Message, tc.wantMsg)
			}
			if se.RecoveryPlaybook != tc.wantRetry {
				t.Errorf("recovery_playbook = %q, want %q", se.RecoveryPlaybook, tc.wantRetry)
			}
			if se.Param != tc.wantParam {
				t.Errorf("param = %q, want %q", se.Param, tc.wantParam)
			}
		})
	}
}
