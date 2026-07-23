// interact_state_coverage_test.go — Behavioural tests for state snapshots, evidence
// capture, browser-storage mutation, and the retry contract.
//
// Covers: interact_storage.go, interact_evidence*.go, interact_state_*.go,
// interact_retry_contract_state.go, interact_retry_contract_response.go.
//
// All package-scope identifiers in this file are prefixed `statecov` to stay
// collision-free with the other test files in this shared package.
//
// These tests are deliberately NOT parallel: several mutate process environment
// (t.Setenv) and the package-global evidenceCaptureFn.

package toolinteract

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// statecovRecordedAction captures one RecordAIAction call.
type statecovRecordedAction struct {
	Action string
	URL    string
	Extra  map[string]any
}

// statecovHarness wires a fully stubbed Deps plus a real on-disk SessionStore
// rooted under t.TempDir(), so state handlers run end to end without a browser.
type statecovHarness struct {
	t      *testing.T
	deps   *Deps
	cap    *capture.Store
	store  *persistence.SessionStore
	state  *StateInteractHandler
	action *InteractActionHandler

	mu       sync.Mutex
	enqueued []queries.PendingQuery
	recorded []statecovRecordedAction

	// Toggles — set before invoking a handler.
	blockEnqueue      bool
	blockSessionStore bool
	blockPilotGuard   bool
	redaction         RedactionEngine

	// autoComplete, when set, resolves each enqueued command on the capture
	// store so WaitForCommand returns immediately.
	autoComplete func(q queries.PendingQuery) (status string, result json.RawMessage, errMsg string)
}

// statecovPassGuard never blocks.
func statecovPassGuard(_ JSONRPCRequest, _ ...func(*StructuredError)) (JSONRPCResponse, bool) {
	return JSONRPCResponse{}, false
}

func statecovNewHarness(t *testing.T) *statecovHarness {
	t.Helper()
	// Confine every persistence write to the test's temp dir.
	t.Setenv(state.StateDirEnv, t.TempDir())

	store, err := persistence.NewSessionStoreWithInterval(t.TempDir(), time.Hour)
	if err != nil {
		t.Fatalf("NewSessionStoreWithInterval: %v", err)
	}
	t.Cleanup(store.Shutdown)

	c := capture.NewCapture()
	t.Cleanup(c.Close)

	hs := &statecovHarness{t: t, cap: c, store: store}
	hs.deps = &Deps{
		RequirePilot: func(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool) {
			if hs.blockPilotGuard {
				return fail(req, ErrCodePilotDisabled, "Pilot disabled", "Enable AI Web Pilot", opts...), true
			}
			return JSONRPCResponse{}, false
		},
		RequireExtension:   statecovPassGuard,
		RequireTabTracking: statecovPassGuard,
		RequireCSPClear: func(_ JSONRPCRequest, _ string) (JSONRPCResponse, bool) {
			return JSONRPCResponse{}, false
		},
		EnqueuePendingQuery: func(req JSONRPCRequest, q queries.PendingQuery, _ time.Duration) (JSONRPCResponse, bool) {
			hs.mu.Lock()
			hs.enqueued = append(hs.enqueued, q)
			hs.mu.Unlock()
			if hs.blockEnqueue {
				return fail(req, ErrQueueFull, "Command queue is full", "Retry shortly"), true
			}
			if hs.autoComplete != nil {
				status, result, errMsg := hs.autoComplete(q)
				c.RegisterCommand(q.CorrelationID, "q-"+q.CorrelationID, 30*time.Second)
				c.ApplyCommandResult(q.CorrelationID, status, result, errMsg)
			}
			return JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req JSONRPCRequest, correlationID string, _ json.RawMessage, queuedSummary string) JSONRPCResponse {
			return succeed(req, queuedSummary, map[string]any{"status": "queued", "correlation_id": correlationID})
		},
		Capture: func() *capture.Store { return c },
		RecordAIAction: func(action, url string, extra map[string]any) {
			hs.mu.Lock()
			hs.recorded = append(hs.recorded, statecovRecordedAction{Action: action, URL: url, Extra: extra})
			hs.mu.Unlock()
		},
		RequireSessionStore: func(req JSONRPCRequest) (JSONRPCResponse, bool) {
			if hs.blockSessionStore {
				return fail(req, ErrNotInitialized, "Session store unavailable", "Restart the daemon"), true
			}
			return JSONRPCResponse{}, false
		},
		DiagnosticHint:     func() func(*StructuredError) { return withHint("statecov-diag") },
		GetRedactionEngine: func() RedactionEngine { return hs.redaction },
	}
	hs.state = NewStateInteractHandler(hs.deps, store)
	hs.action = NewInteractActionHandler(hs.deps)
	return hs
}

func statecovReq() JSONRPCRequest {
	return JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: float64(11), ClientID: "statecov-client"}
}

// statecovQueries returns a copy of every query that reached EnqueuePendingQuery.
func (hs *statecovHarness) statecovQueries() []queries.PendingQuery {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	out := make([]queries.PendingQuery, len(hs.enqueued))
	copy(out, hs.enqueued)
	return out
}

// statecovLastQuery returns the most recently enqueued query, failing if none.
func (hs *statecovHarness) statecovLastQuery() queries.PendingQuery {
	hs.t.Helper()
	all := hs.statecovQueries()
	if len(all) == 0 {
		hs.t.Fatal("expected at least one enqueued query, got none")
	}
	return all[len(all)-1]
}

// statecovLastParams unmarshals the params of the most recently enqueued query.
func (hs *statecovHarness) statecovLastParams() map[string]any {
	hs.t.Helper()
	var m map[string]any
	if err := json.Unmarshal(hs.statecovLastQuery().Params, &m); err != nil {
		hs.t.Fatalf("parse query params: %v", err)
	}
	return m
}

func (hs *statecovHarness) statecovRecords() []statecovRecordedAction {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	out := make([]statecovRecordedAction, len(hs.recorded))
	copy(out, hs.recorded)
	return out
}

// ---------------------------------------------------------------------------
// Response parsing helpers
// ---------------------------------------------------------------------------

func statecovResult(t *testing.T, resp JSONRPCResponse) MCPToolResult {
	t.Helper()
	var out MCPToolResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	return out
}

// statecovPayload extracts the JSON object following the summary line of a
// succeed()/fail() body.
func statecovPayload(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	result := statecovResult(t, resp)
	if len(result.Content) == 0 {
		t.Fatalf("response has no content blocks: %s", string(resp.Result))
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON payload in response text: %q", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse payload %q: %v", text[idx:], err)
	}
	return data
}

// statecovOK asserts a non-error response and returns its payload.
func statecovOK(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	if statecovResult(t, resp).IsError {
		t.Fatalf("expected success, got error: %s", statecovResult(t, resp).Content[0].Text)
	}
	return statecovPayload(t, resp)
}

// statecovFail asserts an error response and returns its structured error payload.
func statecovFail(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	if !statecovResult(t, resp).IsError {
		t.Fatalf("expected error, got success: %s", statecovResult(t, resp).Content[0].Text)
	}
	return statecovPayload(t, resp)
}

// statecovStr reads a string field, failing the test when absent or wrong-typed.
func statecovStr(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("field %q missing or not a string in %v", key, m)
	}
	return v
}

// statecovSub asserts that haystack contains needle.
func statecovSub(t *testing.T, haystack, needle, what string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%s: expected to contain %q, got %q", what, needle, haystack)
	}
}

// ===========================================================================
// interact_storage.go — storage / cookie mutation handlers
// ===========================================================================

func TestStateCov_SetStorage_BuildsExactSetItemScript(t *testing.T) {
	hs := statecovNewHarness(t)

	resp := hs.action.HandleSetStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage","key":"theme","value":"dark"}`))
	statecovOK(t, resp)

	q := hs.statecovLastQuery()
	if q.Type != "execute" {
		t.Errorf("query type = %q, want execute", q.Type)
	}
	if !strings.HasPrefix(q.CorrelationID, "storage_set_") {
		t.Errorf("correlation id = %q, want storage_set_ prefix", q.CorrelationID)
	}
	params := hs.statecovLastParams()
	want := `(() => { try { localStorage.setItem("theme", "dark"); return { ok: true, action: "set_storage", storage_type: "localStorage", key: "theme" }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`
	if got := params["script"]; got != want {
		t.Errorf("script =\n%v\nwant\n%v", got, want)
	}
	if params["reason"] != "set_storage" {
		t.Errorf("reason = %v, want set_storage", params["reason"])
	}
}

// Pins the jsQuote escaping boundary: an unescaped key/value would let a caller
// break out of the JS string literal and inject arbitrary code into the page.
func TestStateCov_SetStorage_EscapesQuotesAndAngleBrackets(t *testing.T) {
	hs := statecovNewHarness(t)

	args := json.RawMessage(`{"storage_type":"sessionStorage","key":"a\"b","value":"</script><img>"}`)
	statecovOK(t, hs.action.HandleSetStorage(statecovReq(), args))

	script, _ := hs.statecovLastParams()["script"].(string)
	statecovSub(t, script, `sessionStorage.setItem("a\"b", "\u003c/script\u003e\u003cimg\u003e")`, "escaped setItem call")
	if strings.Contains(script, "</script>") {
		t.Errorf("raw </script> leaked into the page script, breaking out of the JS string: %q", script)
	}
}

func TestStateCov_SetStorage_RejectsUnknownStorageType(t *testing.T) {
	hs := statecovNewHarness(t)

	resp := hs.action.HandleSetStorage(statecovReq(), json.RawMessage(`{"storage_type":"cookieJar","key":"k","value":"v"}`))
	payload := statecovFail(t, resp)

	if got := statecovStr(t, payload, "error_code"); got != ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", got, ErrInvalidParam)
	}
	if got := statecovStr(t, payload, "param"); got != "storage_type" {
		t.Errorf("param = %q, want storage_type", got)
	}
	statecovSub(t, statecovStr(t, payload, "message"), "cookieJar", "message echoes the bad value")
	if n := len(hs.statecovQueries()); n != 0 {
		t.Errorf("enqueued %d queries on validation failure, want 0", n)
	}
}

func TestStateCov_SetStorage_MissingKeyAndMissingValueAreDistinctErrors(t *testing.T) {
	hs := statecovNewHarness(t)

	noKey := statecovFail(t, hs.action.HandleSetStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage","value":"v"}`)))
	if got := statecovStr(t, noKey, "param"); got != "key" {
		t.Errorf("missing-key param = %q, want key", got)
	}
	if got := statecovStr(t, noKey, "error_code"); got != ErrMissingParam {
		t.Errorf("missing-key error_code = %q, want %q", got, ErrMissingParam)
	}

	noValue := statecovFail(t, hs.action.HandleSetStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k"}`)))
	if got := statecovStr(t, noValue, "param"); got != "value" {
		t.Errorf("missing-value param = %q, want value", got)
	}
}

// An empty string is a legitimate storage value; only an absent "value" is an error.
func TestStateCov_SetStorage_EmptyStringValueIsAccepted(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleSetStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":""}`)))

	script, _ := hs.statecovLastParams()["script"].(string)
	statecovSub(t, script, `localStorage.setItem("k", "")`, "empty value preserved")
}

func TestStateCov_SetStorage_MalformedJSONArgs(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.action.HandleSetStorage(statecovReq(), json.RawMessage(`{"storage_type":`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", got, ErrInvalidJSON)
	}
}

func TestStateCov_DeleteStorage_BuildsRemoveItemScript(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleDeleteStorage(statecovReq(), json.RawMessage(`{"storage_type":"sessionStorage","key":"tok"}`)))

	q := hs.statecovLastQuery()
	if !strings.HasPrefix(q.CorrelationID, "storage_del_") {
		t.Errorf("correlation id = %q, want storage_del_ prefix", q.CorrelationID)
	}
	want := `(() => { try { sessionStorage.removeItem("tok"); return { ok: true, action: "delete_storage", storage_type: "sessionStorage", key: "tok" }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`
	if got := hs.statecovLastParams()["script"]; got != want {
		t.Errorf("script =\n%v\nwant\n%v", got, want)
	}
}

func TestStateCov_DeleteStorage_RequiresKeyAndValidType(t *testing.T) {
	hs := statecovNewHarness(t)

	badType := statecovFail(t, hs.action.HandleDeleteStorage(statecovReq(), json.RawMessage(`{"storage_type":"nope","key":"k"}`)))
	if got := statecovStr(t, badType, "param"); got != "storage_type" {
		t.Errorf("param = %q, want storage_type", got)
	}

	noKey := statecovFail(t, hs.action.HandleDeleteStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage"}`)))
	if got := statecovStr(t, noKey, "param"); got != "key" {
		t.Errorf("param = %q, want key", got)
	}
}

// clear_storage takes no key — the key guard must not fire.
func TestStateCov_ClearStorage_NeedsOnlyStorageType(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleClearStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage"}`)))

	want := `(() => { try { localStorage.clear(); return { ok: true, action: "clear_storage", storage_type: "localStorage" }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`
	if got := hs.statecovLastParams()["script"]; got != want {
		t.Errorf("script =\n%v\nwant\n%v", got, want)
	}
	if !strings.HasPrefix(hs.statecovLastQuery().CorrelationID, "storage_clear_") {
		t.Errorf("correlation id = %q, want storage_clear_ prefix", hs.statecovLastQuery().CorrelationID)
	}
}

func TestStateCov_ClearStorage_RejectsUnknownType(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.action.HandleClearStorage(statecovReq(), json.RawMessage(`{"storage_type":"indexedDB"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", got, ErrInvalidParam)
	}
}

// path defaults to "/" so a cookie set from a deep URL is still visible site-wide.
func TestStateCov_SetCookie_DefaultsPathToRoot(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleSetCookie(statecovReq(), json.RawMessage(`{"name":"sid","value":"abc"}`)))

	script, _ := hs.statecovLastParams()["script"].(string)
	statecovSub(t, script, `document.cookie = "sid=abc; path=/"`, "default cookie path")
	if !strings.HasPrefix(hs.statecovLastQuery().CorrelationID, "cookie_set_") {
		t.Errorf("correlation id = %q, want cookie_set_ prefix", hs.statecovLastQuery().CorrelationID)
	}
}

func TestStateCov_SetCookie_AppendsExplicitPathThenDomain(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleSetCookie(statecovReq(),
		json.RawMessage(`{"name":"sid","value":"abc","path":"/app","domain":".example.com"}`)))

	script, _ := hs.statecovLastParams()["script"].(string)
	statecovSub(t, script, `document.cookie = "sid=abc; path=/app; domain=.example.com"`, "cookie attribute order")
}

func TestStateCov_SetCookie_RequiresNameAndValue(t *testing.T) {
	hs := statecovNewHarness(t)

	noName := statecovFail(t, hs.action.HandleSetCookie(statecovReq(), json.RawMessage(`{"value":"v"}`)))
	if got := statecovStr(t, noName, "param"); got != "name" {
		t.Errorf("param = %q, want name", got)
	}

	noValue := statecovFail(t, hs.action.HandleSetCookie(statecovReq(), json.RawMessage(`{"name":"sid"}`)))
	if got := statecovStr(t, noValue, "param"); got != "value" {
		t.Errorf("param = %q, want value", got)
	}
	statecovSub(t, statecovStr(t, noValue, "message"), "set_cookie", "message names the action")
}

// Deletion works by setting an already-expired cookie; the epoch date is the mechanism.
func TestStateCov_DeleteCookie_UsesEpochExpiry(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleDeleteCookie(statecovReq(), json.RawMessage(`{"name":"sid","domain":"example.com"}`)))

	want := `(() => { try { document.cookie = "sid=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/; domain=example.com"; return { ok: true, action: "delete_cookie", name: "sid" }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`
	if got := hs.statecovLastParams()["script"]; got != want {
		t.Errorf("script =\n%v\nwant\n%v", got, want)
	}
	if !strings.HasPrefix(hs.statecovLastQuery().CorrelationID, "cookie_del_") {
		t.Errorf("correlation id = %q, want cookie_del_ prefix", hs.statecovLastQuery().CorrelationID)
	}
}

func TestStateCov_DeleteCookie_RequiresName(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.action.HandleDeleteCookie(statecovReq(), json.RawMessage(`{"path":"/x"}`)))
	if got := statecovStr(t, payload, "param"); got != "name" {
		t.Errorf("param = %q, want name", got)
	}
}

func TestStateCov_QueueExecuteScript_DefaultsWorldAndTimeout(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleClearStorage(statecovReq(), json.RawMessage(`{"storage_type":"localStorage","timeout_ms":-5}`)))

	params := hs.statecovLastParams()
	if params["world"] != "auto" {
		t.Errorf("world = %v, want auto", params["world"])
	}
	if params["timeout_ms"] != float64(5000) {
		t.Errorf("timeout_ms = %v, want 5000 (non-positive input must be replaced)", params["timeout_ms"])
	}
}

func TestStateCov_QueueExecuteScript_HonoursExplicitWorldAndTimeout(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleClearStorage(statecovReq(),
		json.RawMessage(`{"storage_type":"localStorage","world":"isolated","timeout_ms":1200,"tab_id":42}`)))

	params := hs.statecovLastParams()
	if params["world"] != "isolated" {
		t.Errorf("world = %v, want isolated", params["world"])
	}
	if params["timeout_ms"] != float64(1200) {
		t.Errorf("timeout_ms = %v, want 1200", params["timeout_ms"])
	}
	if got := hs.statecovLastQuery().TabID; got != 42 {
		t.Errorf("tab_id = %d, want 42", got)
	}
}

func TestStateCov_QueueExecuteScript_RejectsUnknownWorld(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.action.HandleClearStorage(statecovReq(),
		json.RawMessage(`{"storage_type":"localStorage","world":"universe"}`)))
	if got := statecovStr(t, payload, "param"); got != "world" {
		t.Errorf("param = %q, want world", got)
	}
	statecovSub(t, statecovStr(t, payload, "message"), "universe", "message echoes the bad world")
	if n := len(hs.statecovQueries()); n != 0 {
		t.Errorf("enqueued %d queries for an invalid world, want 0", n)
	}
}

func TestStateCov_QueueExecuteScript_PilotGuardBlocksBeforeEnqueue(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.blockPilotGuard = true

	payload := statecovFail(t, hs.action.HandleSetStorage(statecovReq(),
		json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrCodePilotDisabled {
		t.Errorf("error_code = %q, want %q", got, ErrCodePilotDisabled)
	}
	if n := len(hs.statecovQueries()); n != 0 {
		t.Errorf("enqueued %d queries while pilot was disabled, want 0", n)
	}
}

func TestStateCov_ValidateStorageType_MapsNameToJSExpression(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"localStorage", "localStorage"},
		{"sessionStorage", "sessionStorage"},
	} {
		expr, _, ok := validateStorageType(statecovReq(), tc.in)
		if !ok || expr != tc.want {
			t.Errorf("validateStorageType(%q) = (%q,%v), want (%q,true)", tc.in, expr, ok, tc.want)
		}
	}
	// Case sensitivity is deliberate: "localstorage" is not a valid JS global.
	if _, _, ok := validateStorageType(statecovReq(), "localstorage"); ok {
		t.Error("validateStorageType must be case-sensitive")
	}
	if _, _, ok := validateStorageType(statecovReq(), ""); ok {
		t.Error("empty storage_type must be rejected")
	}
}

func TestStateCov_JSQuote_ProducesHTMLSafeJSONStrings(t *testing.T) {
	// HTML-escaping (< > &) is on by default in encoding/json and is what keeps a
	// storage value from terminating an inline <script> block.
	cases := map[string]string{
		"plain":     `"plain"`,
		`a"b`:       `"a\"b"`,
		"line\nend": `"line\nend"`,
		`</script>`: `"\u003c/script\u003e"`,
		`a&b`:       `"a\u0026b"`,
		"":          `""`,
	}
	for in, want := range cases {
		if got := jsQuote(in); got != want {
			t.Errorf("jsQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

// ===========================================================================
// interact_evidence_config.go — evidence mode + env-bounded config parsing
// ===========================================================================

func TestStateCov_ParseEvidenceMode_AcceptedValues(t *testing.T) {
	cases := []struct {
		args string
		want evidenceMode
	}{
		{`{}`, evidenceModeOff},
		{`{"evidence":""}`, evidenceModeOff},
		{`{"evidence":"   "}`, evidenceModeOff},
		{`{"evidence":"off"}`, evidenceModeOff},
		{`{"evidence":"OFF"}`, evidenceModeOff},
		{`{"evidence":"on_mutation"}`, evidenceModeOnMutation},
		{`{"evidence":" On_Mutation "}`, evidenceModeOnMutation},
		{`{"evidence":"always"}`, evidenceModeAlways},
	}
	for _, tc := range cases {
		got, err := ParseEvidenceMode(json.RawMessage(tc.args))
		if err != nil {
			t.Errorf("ParseEvidenceMode(%s) unexpected error: %v", tc.args, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseEvidenceMode(%s) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestStateCov_ParseEvidenceMode_UnknownValueErrorsAndFallsBackToOff(t *testing.T) {
	got, err := ParseEvidenceMode(json.RawMessage(`{"evidence":"screenshot"}`))
	if err == nil {
		t.Fatal("expected an error for an unknown evidence mode")
	}
	if got != evidenceModeOff {
		t.Errorf("mode = %q, want off on error", got)
	}
	statecovSub(t, err.Error(), "screenshot", "error echoes the rejected value")
	statecovSub(t, err.Error(), "off, on_mutation, always", "error lists valid modes")
}

// Evidence config is optional: a malformed or wrong-typed payload must degrade
// to "off" rather than failing the whole interact call.
func TestStateCov_ParseEvidenceMode_MalformedArgsDegradeToOff(t *testing.T) {
	for _, args := range []string{`{"evidence":`, `{"evidence":7}`, ``, `null`} {
		got, err := ParseEvidenceMode(json.RawMessage(args))
		if err != nil {
			t.Errorf("ParseEvidenceMode(%q) error = %v, want nil", args, err)
		}
		if got != evidenceModeOff {
			t.Errorf("ParseEvidenceMode(%q) = %q, want off", args, got)
		}
	}
}

func TestStateCov_ParseBoundedEnvInt_ClampsAndFallsBack(t *testing.T) {
	const envName = "KABOOM_STATECOV_BOUNDED"
	cases := []struct {
		raw  string
		set  bool
		want int
	}{
		{"", false, 5},   // unset -> default
		{"  ", true, 5},  // blank -> default
		{"abc", true, 5}, // unparseable -> default
		{"1", true, 2},   // below min -> min
		{"2", true, 2},
		{"7", true, 7},
		{"9", true, 8},    // above max -> max
		{"-4", true, 2},   // below min -> min
		{"  6 ", true, 6}, // trimmed
	}
	for _, tc := range cases {
		if tc.set {
			t.Setenv(envName, tc.raw)
		} else {
			t.Setenv(envName, "")
		}
		if got := parseBoundedEnvInt(envName, 5, 2, 8); got != tc.want {
			t.Errorf("parseBoundedEnvInt(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestStateCov_EvidenceMaxCaptures_DefaultsToTwoAndClampsToZeroTwo(t *testing.T) {
	t.Setenv(evidenceMaxCapturesEnv, "")
	if got := evidenceMaxCapturesPerCommand(); got != 2 {
		t.Errorf("default max captures = %d, want 2", got)
	}
	t.Setenv(evidenceMaxCapturesEnv, "0")
	if got := evidenceMaxCapturesPerCommand(); got != 0 {
		t.Errorf("max captures with env=0 is %d, want 0 (evidence must be disableable)", got)
	}
	t.Setenv(evidenceMaxCapturesEnv, "99")
	if got := evidenceMaxCapturesPerCommand(); got != 2 {
		t.Errorf("max captures with env=99 is %d, want 2 (hard upper bound)", got)
	}
}

func TestStateCov_EvidenceRetryCount_DefaultsToOneAndClampsToZeroThree(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "")
	if got := evidenceRetryCount(); got != 1 {
		t.Errorf("default retry count = %d, want 1", got)
	}
	t.Setenv(evidenceRetryEnv, "-2")
	if got := evidenceRetryCount(); got != 0 {
		t.Errorf("retry count with env=-2 is %d, want 0", got)
	}
	t.Setenv(evidenceRetryEnv, "10")
	if got := evidenceRetryCount(); got != 3 {
		t.Errorf("retry count with env=10 is %d, want 3 (hard upper bound)", got)
	}
}

func TestStateCov_CanonicalActionFromInteractArgs_PrefersWhatOverAction(t *testing.T) {
	cases := []struct {
		args string
		want string
	}{
		{`{"what":" Click ","action":"type"}`, "click"},
		{`{"action":"NAVIGATE"}`, "navigate"},
		{`{"what":"","action":"type"}`, "type"},
		{`{"what":"   ","action":"type"}`, "type"},
		{`{}`, ""},
		{`{"what":`, ""},
	}
	for _, tc := range cases {
		if got := canonicalActionFromInteractArgs(json.RawMessage(tc.args)); got != tc.want {
			t.Errorf("canonicalActionFromInteractArgs(%s) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestStateCov_IsMutationAction_SeparatesReadsFromWrites(t *testing.T) {
	mutating := []string{"click", "type", "navigate", "set_storage", "delete_cookie",
		"fill_form_and_submit", "upload", "execute_js", "highlight", " CLICK "}
	for _, a := range mutating {
		if !isMutationAction(a) {
			t.Errorf("isMutationAction(%q) = false, want true", a)
		}
	}
	// "clicked" must not match "click": the switch is exact, not a prefix test.
	nonMutating := []string{"", "screenshot", "get_text", "list_interactive", "query",
		"wait_for", "explore_page", "clicked", "click_all"}
	for _, a := range nonMutating {
		if isMutationAction(a) {
			t.Errorf("isMutationAction(%q) = true, want false", a)
		}
	}
}

// ===========================================================================
// interact_evidence_state.go — payload assembly, cloning, state lifecycle
// ===========================================================================

func TestStateCov_BuildEvidencePayload_NilStateYieldsEmptyMap(t *testing.T) {
	got := buildEvidencePayload(nil)
	if got == nil {
		t.Fatal("buildEvidencePayload(nil) = nil, want empty map (callers index it directly)")
	}
	if len(got) != 0 {
		t.Errorf("buildEvidencePayload(nil) = %v, want empty map", got)
	}
}

func TestStateCov_BuildEvidencePayload_BothShotsSucceeded(t *testing.T) {
	got := buildEvidencePayload(&commandEvidenceState{
		mode:   evidenceModeAlways,
		action: "click",
		before: evidenceShot{Path: "/tmp/b.png", Filename: "b.png"},
		after:  evidenceShot{Path: "/tmp/a.png", Filename: "a.png"},
	})

	if got["mode"] != "always" || got["action"] != "click" {
		t.Errorf("mode/action = %v/%v, want always/click", got["mode"], got["action"])
	}
	if got["before"] != "/tmp/b.png" || got["after"] != "/tmp/a.png" {
		t.Errorf("before/after = %v/%v, want the shot paths", got["before"], got["after"])
	}
	files, _ := got["filenames"].(map[string]any)
	if files["before"] != "b.png" || files["after"] != "a.png" {
		t.Errorf("filenames = %v, want b.png/a.png", got["filenames"])
	}
	if _, has := got["errors"]; has {
		t.Errorf("errors must be absent when both shots succeeded: %v", got["errors"])
	}
	if _, has := got["partial"]; has {
		t.Error("partial must be absent when nothing failed")
	}
}

// "partial" is the signal that some proof exists but the pair is incomplete;
// it must not appear when every capture failed (no artefact at all).
func TestStateCov_BuildEvidencePayload_PartialOnlyWhenOneShotLanded(t *testing.T) {
	half := buildEvidencePayload(&commandEvidenceState{
		mode:   evidenceModeOnMutation,
		action: "type",
		before: evidenceShot{Path: "/tmp/b.png", Filename: "b.png"},
		after:  evidenceShot{Error: "screenshot_timeout"},
	})
	if half["partial"] != true {
		t.Errorf("partial = %v, want true when before landed and after failed", half["partial"])
	}
	errs, _ := half["errors"].(map[string]any)
	if errs["after"] != "screenshot_timeout" {
		t.Errorf("errors = %v, want after=screenshot_timeout", half["errors"])
	}
	if _, has := errs["before"]; has {
		t.Error("errors must not carry a before entry when before succeeded")
	}

	none := buildEvidencePayload(&commandEvidenceState{
		mode:   evidenceModeAlways,
		action: "click",
		before: evidenceShot{Error: "e1"},
		after:  evidenceShot{Error: "e2"},
	})
	if _, has := none["partial"]; has {
		t.Errorf("partial must be absent when no screenshot landed: %v", none)
	}
	if _, has := none["filenames"]; has {
		t.Errorf("filenames must be absent when no shot produced a file: %v", none["filenames"])
	}
}

func TestStateCov_BuildEvidencePayload_SkipReasonIsSurfaced(t *testing.T) {
	got := buildEvidencePayload(&commandEvidenceState{
		mode:    evidenceModeOnMutation,
		action:  "get_text",
		skipped: "non_mutating_action",
	})
	if got["skipped"] != "non_mutating_action" {
		t.Errorf("skipped = %v, want non_mutating_action", got["skipped"])
	}
	if _, has := got["before"]; has {
		t.Error("before must be absent when nothing was captured")
	}
}

func TestStateCov_CloneAnyMap_DeepCopiesNestedMaps(t *testing.T) {
	if cloneAnyMap(nil) != nil {
		t.Error("cloneAnyMap(nil) must stay nil so callers can distinguish 'no payload'")
	}

	orig := map[string]any{
		"mode":      "always",
		"filenames": map[string]any{"before": "b.png"},
	}
	clone := cloneAnyMap(orig)

	clone["mode"] = "off"
	clone["filenames"].(map[string]any)["before"] = "MUTATED"

	if orig["mode"] != "always" {
		t.Errorf("top-level mutation leaked into the original: %v", orig["mode"])
	}
	if orig["filenames"].(map[string]any)["before"] != "b.png" {
		t.Errorf("nested mutation leaked into the cached payload: %v", orig["filenames"])
	}
}

func TestStateCov_EvidenceStateLifecycle_StoreLoadFinalizeClear(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})

	// Unknown correlation ID is "done" with no cached payload.
	cached, needsAfter, clientID, done := h.loadEvidenceAttachContext("nope")
	if !done || cached != nil || needsAfter || clientID != "" {
		t.Fatalf("unknown id => (%v,%v,%q,%v), want (nil,false,\"\",true)", cached, needsAfter, clientID, done)
	}
	if _, ok := h.finalizeEvidencePayload("nope", false, evidenceShot{}); ok {
		t.Error("finalizeEvidencePayload must report not-found for an unknown correlation id")
	}

	h.storeEvidenceState("c1", &commandEvidenceState{
		mode:          evidenceModeAlways,
		action:        "click",
		shouldCapture: true,
		maxCaptures:   2,
		clientID:      "client-x",
		before:        evidenceShot{Path: "/b.png", Filename: "b.png"},
	})

	cached, needsAfter, clientID, done = h.loadEvidenceAttachContext("c1")
	if done || cached != nil {
		t.Fatalf("armed state must not be done yet: cached=%v done=%v", cached, done)
	}
	if !needsAfter {
		t.Error("needsAfter must be true when shouldCapture && maxCaptures > 1")
	}
	if clientID != "client-x" {
		t.Errorf("clientID = %q, want client-x", clientID)
	}

	payload, ok := h.finalizeEvidencePayload("c1", true, evidenceShot{Path: "/a.png", Filename: "a.png"})
	if !ok {
		t.Fatal("finalizeEvidencePayload returned not-found for an armed command")
	}
	if payload["after"] != "/a.png" {
		t.Errorf("after = %v, want /a.png", payload["after"])
	}

	// Once finalized the cached payload is replayed verbatim and no further
	// capture is requested.
	cached, needsAfter, _, done = h.loadEvidenceAttachContext("c1")
	if !done || needsAfter {
		t.Errorf("finalized state => done=%v needsAfter=%v, want true/false", done, needsAfter)
	}
	if cached["before"] != "/b.png" || cached["after"] != "/a.png" {
		t.Errorf("cached payload = %v, want the finalized before/after paths", cached)
	}

	// The replayed payload is a copy; mutating it must not corrupt the cache.
	cached["before"] = "MUTATED"
	again, _, _, _ := h.loadEvidenceAttachContext("c1")
	if again["before"] != "/b.png" {
		t.Errorf("cached payload was mutated through a returned copy: %v", again["before"])
	}

	h.clearEvidenceState("c1")
	if _, ok := h.finalizeEvidencePayload("c1", false, evidenceShot{}); ok {
		t.Error("clearEvidenceState did not remove the state")
	}
}

// A second finalize must not overwrite the first result — response decoration
// can run more than once for the same command.
func TestStateCov_FinalizeEvidencePayload_IsIdempotent(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.storeEvidenceState("c1", &commandEvidenceState{
		mode:   evidenceModeAlways,
		action: "click",
		before: evidenceShot{Path: "/b.png"},
	})

	first, _ := h.finalizeEvidencePayload("c1", true, evidenceShot{Path: "/a1.png"})
	second, _ := h.finalizeEvidencePayload("c1", true, evidenceShot{Path: "/a2.png"})

	if first["after"] != "/a1.png" {
		t.Fatalf("first after = %v, want /a1.png", first["after"])
	}
	if second["after"] != "/a1.png" {
		t.Errorf("second after = %v, want /a1.png (finalized payload must be frozen)", second["after"])
	}
}

func TestStateCov_StoreEvidenceState_InitializesNilMap(t *testing.T) {
	h := &InteractActionHandler{deps: &Deps{}}
	h.storeEvidenceState("c1", &commandEvidenceState{mode: evidenceModeAlways, action: "click"})
	if _, ok := h.finalizeEvidencePayload("c1", false, evidenceShot{}); !ok {
		t.Error("storeEvidenceState must lazily create evidenceByCommand")
	}
}

// ===========================================================================
// interact_evidence_capture.go + interact_evidence.go — arm / capture / attach
// ===========================================================================

// statecovCaptureRecorder installs a fake evidence capture function for the
// duration of the test and records every call.
type statecovCaptureRecorder struct {
	mu       sync.Mutex
	clients  []string
	shots    []evidenceShot
	nextShot func(n int) evidenceShot
}

func statecovInstallCapture(t *testing.T, next func(n int) evidenceShot) *statecovCaptureRecorder {
	t.Helper()
	rec := &statecovCaptureRecorder{nextShot: next}
	SetEvidenceCaptureFn(func(_ *Deps, clientID string) EvidenceShot {
		rec.mu.Lock()
		n := len(rec.clients)
		rec.clients = append(rec.clients, clientID)
		rec.mu.Unlock()
		shot := rec.nextShot(n)
		rec.mu.Lock()
		rec.shots = append(rec.shots, shot)
		rec.mu.Unlock()
		return shot
	})
	t.Cleanup(ResetEvidenceCaptureFn)
	return rec
}

func (r *statecovCaptureRecorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

func (r *statecovCaptureRecorder) clientAt(i int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i >= len(r.clients) {
		return ""
	}
	return r.clients[i]
}

func TestStateCov_CaptureEvidenceWithRetry_NoCaptureFnConfigured(t *testing.T) {
	ResetEvidenceCaptureFn()
	h := NewInteractActionHandler(&Deps{})

	shot := h.captureEvidenceWithRetry("client-a")
	if shot.Error != "evidence_capture_not_configured" {
		t.Errorf("error = %q, want evidence_capture_not_configured", shot.Error)
	}
	if shot.Attempts != 0 {
		t.Errorf("attempts = %d, want 0 (nothing was attempted)", shot.Attempts)
	}
}

// With no global override installed, the handler must fall back to the
// injected DefaultEvidenceCapture dependency.
func TestStateCov_CaptureEvidenceWithRetry_FallsBackToDepsCapture(t *testing.T) {
	ResetEvidenceCaptureFn()
	t.Setenv(evidenceRetryEnv, "0")

	var gotClient string
	h := NewInteractActionHandler(&Deps{
		DefaultEvidenceCapture: func(clientID string) EvidenceShot {
			gotClient = clientID
			return EvidenceShot{Path: "/shots/x.png", Filename: "x.png"}
		},
	})

	shot := h.captureEvidenceWithRetry("client-b")
	if shot.Path != "/shots/x.png" || shot.Filename != "x.png" {
		t.Errorf("shot = %+v, want the deps-provided screenshot", shot)
	}
	if shot.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", shot.Attempts)
	}
	if gotClient != "client-b" {
		t.Errorf("client id passed through = %q, want client-b", gotClient)
	}
}

func TestStateCov_CaptureEvidenceWithRetry_RetriesUntilAPathAppears(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "1") // 1 retry => 2 attempts
	rec := statecovInstallCapture(t, func(n int) evidenceShot {
		if n == 0 {
			return evidenceShot{Error: "transient"}
		}
		return evidenceShot{Path: "/shots/second.png"}
	})
	h := NewInteractActionHandler(&Deps{})

	shot := h.captureEvidenceWithRetry("client-c")
	if shot.Path != "/shots/second.png" {
		t.Errorf("path = %q, want the second attempt's shot", shot.Path)
	}
	if shot.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", shot.Attempts)
	}
	if rec.calls() != 2 {
		t.Errorf("capture called %d times, want 2", rec.calls())
	}
	if rec.clientAt(1) != "client-c" {
		t.Errorf("retry used client id %q, want client-c", rec.clientAt(1))
	}
}

// A capture that returns neither a path nor an error must still surface a
// non-empty failure reason, otherwise the evidence payload is silently empty.
func TestStateCov_CaptureEvidenceWithRetry_SynthesizesFailureReason(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "0") // single attempt, no retry delay
	rec := statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{} })
	h := NewInteractActionHandler(&Deps{})

	shot := h.captureEvidenceWithRetry("client-d")
	if shot.Error != "evidence_capture_failed" {
		t.Errorf("error = %q, want evidence_capture_failed", shot.Error)
	}
	if shot.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", shot.Attempts)
	}
	if rec.calls() != 1 {
		t.Errorf("capture called %d times, want 1 with retry count 0", rec.calls())
	}
}

// A whitespace-only path does not count as a captured screenshot.
func TestStateCov_CaptureEvidenceWithRetry_BlankPathIsNotSuccess(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "0")
	statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "   ", Error: "weird"} })
	h := NewInteractActionHandler(&Deps{})

	shot := h.captureEvidenceWithRetry("client-e")
	if shot.Error != "weird" {
		t.Errorf("error = %q, want the reported error preserved", shot.Error)
	}
	if strings.TrimSpace(shot.Path) != "" {
		t.Errorf("path = %q, want blank treated as failure", shot.Path)
	}
}

func TestStateCov_ArmEvidence_IgnoresEmptyCorrelationAndNilHandler(t *testing.T) {
	statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/x.png"} })

	var nilHandler *InteractActionHandler
	nilHandler.ArmEvidenceForCommand("c1", "click", json.RawMessage(`{"evidence":"always"}`), "cli")

	h := NewInteractActionHandler(&Deps{})
	h.ArmEvidenceForCommand("", "click", json.RawMessage(`{"evidence":"always"}`), "cli")
	if _, ok := h.getRetryState(""); ok {
		t.Error("an empty correlation id must not create any state")
	}
}

// Evidence "off" clears any previously armed state so a re-used correlation ID
// cannot leak screenshots from an earlier command.
func TestStateCov_ArmEvidence_OffClearsPreviouslyArmedState(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "0")
	statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/x.png"} })
	h := NewInteractActionHandler(&Deps{})

	h.ArmEvidenceForCommand("c1", "click", json.RawMessage(`{"evidence":"always"}`), "cli")
	if _, ok := h.finalizeEvidencePayload("c1", false, evidenceShot{}); !ok {
		t.Fatal("expected evidence state after arming with always")
	}

	h2 := NewInteractActionHandler(&Deps{})
	h2.storeEvidenceState("c2", &commandEvidenceState{mode: evidenceModeAlways})
	h2.ArmEvidenceForCommand("c2", "click", json.RawMessage(`{"evidence":"off"}`), "cli")
	if _, ok := h2.finalizeEvidencePayload("c2", false, evidenceShot{}); ok {
		t.Error("evidence=off must delete the armed state")
	}
}

// An unparseable evidence mode leaves state untouched rather than arming a
// capture the caller never asked for.
func TestStateCov_ArmEvidence_InvalidModeLeavesNoState(t *testing.T) {
	rec := statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/x.png"} })
	h := NewInteractActionHandler(&Deps{})

	h.ArmEvidenceForCommand("c1", "click", json.RawMessage(`{"evidence":"maybe"}`), "cli")

	if _, ok := h.finalizeEvidencePayload("c1", false, evidenceShot{}); ok {
		t.Error("invalid evidence mode must not arm evidence state")
	}
	if rec.calls() != 0 {
		t.Errorf("capture called %d times for an invalid mode, want 0", rec.calls())
	}
	// The retry contract is armed before evidence parsing and must survive.
	if _, ok := h.getRetryState("c1"); !ok {
		t.Error("retry contract must still be armed when evidence config is invalid")
	}
}

func TestStateCov_ArmEvidence_OnMutationSkipsReadOnlyActions(t *testing.T) {
	rec := statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/x.png"} })
	h := NewInteractActionHandler(&Deps{})

	h.ArmEvidenceForCommand("c1", "", json.RawMessage(`{"evidence":"on_mutation","what":"get_text"}`), "cli")

	payload, ok := h.finalizeEvidencePayload("c1", false, evidenceShot{})
	if !ok {
		t.Fatal("expected evidence state to be stored even when the capture is skipped")
	}
	if payload["skipped"] != "non_mutating_action" {
		t.Errorf("skipped = %v, want non_mutating_action", payload["skipped"])
	}
	if payload["action"] != "get_text" {
		t.Errorf("action = %v, want get_text (derived from args)", payload["action"])
	}
	if rec.calls() != 0 {
		t.Errorf("capture called %d times for a read-only action, want 0", rec.calls())
	}
}

func TestStateCov_ArmEvidence_OnMutationCapturesForMutatingActions(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "0")
	rec := statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/before.png", Filename: "before.png"} })
	h := NewInteractActionHandler(&Deps{})

	h.ArmEvidenceForCommand("c1", "", json.RawMessage(`{"evidence":"on_mutation","what":"CLICK"}`), "cli-9")

	payload, _ := h.finalizeEvidencePayload("c1", false, evidenceShot{})
	if payload["before"] != "/before.png" {
		t.Errorf("before = %v, want /before.png", payload["before"])
	}
	if payload["mode"] != "on_mutation" {
		t.Errorf("mode = %v, want on_mutation", payload["mode"])
	}
	if rec.calls() != 1 {
		t.Errorf("capture called %d times, want 1 (before shot only)", rec.calls())
	}
	if rec.clientAt(0) != "cli-9" {
		t.Errorf("capture client = %q, want cli-9", rec.clientAt(0))
	}
}

// A zero capture budget must disable evidence entirely and say why.
func TestStateCov_ArmEvidence_ZeroCaptureBudgetSkips(t *testing.T) {
	t.Setenv(evidenceMaxCapturesEnv, "0")
	rec := statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/x.png"} })
	h := NewInteractActionHandler(&Deps{})

	h.ArmEvidenceForCommand("c1", "click", json.RawMessage(`{"evidence":"always"}`), "cli")

	payload, _ := h.finalizeEvidencePayload("c1", false, evidenceShot{})
	if payload["skipped"] != "capture_budget_zero" {
		t.Errorf("skipped = %v, want capture_budget_zero", payload["skipped"])
	}
	if rec.calls() != 0 {
		t.Errorf("capture called %d times with a zero budget, want 0", rec.calls())
	}
}

func TestStateCov_AttachEvidencePayload_NoOpsWithoutStateOrTarget(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})

	var nilHandler *InteractActionHandler
	nilHandler.AttachEvidencePayload("c1", map[string]any{})

	h.AttachEvidencePayload("", map[string]any{})
	h.AttachEvidencePayload("c1", nil)

	data := map[string]any{"status": "complete"}
	h.AttachEvidencePayload("unknown", data)
	if _, has := data["evidence"]; has {
		t.Errorf("unknown correlation id must not add an evidence key: %v", data)
	}
}

func TestStateCov_AttachEvidencePayload_CapturesAfterShotOnce(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "0")
	t.Setenv(evidenceMaxCapturesEnv, "2")
	shots := []string{"/before.png", "/after.png", "/extra.png"}
	rec := statecovInstallCapture(t, func(n int) evidenceShot {
		return evidenceShot{Path: shots[n], Filename: strings.TrimPrefix(shots[n], "/")}
	})
	h := NewInteractActionHandler(&Deps{})
	h.ArmEvidenceForCommand("c1", "click", json.RawMessage(`{"evidence":"always"}`), "cli")

	first := map[string]any{"status": "complete"}
	h.AttachEvidencePayload("c1", first)

	ev, ok := first["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence missing from response data: %v", first)
	}
	if ev["before"] != "/before.png" || ev["after"] != "/after.png" {
		t.Errorf("evidence = %v, want before=/before.png after=/after.png", ev)
	}

	// Decorating a second response must replay the cached payload without
	// taking another screenshot.
	second := map[string]any{"status": "complete"}
	h.AttachEvidencePayload("c1", second)
	if rec.calls() != 2 {
		t.Errorf("capture called %d times across two attaches, want 2", rec.calls())
	}
	ev2, _ := second["evidence"].(map[string]any)
	if ev2["after"] != "/after.png" {
		t.Errorf("replayed evidence = %v, want the cached after path", ev2)
	}
}

// With a capture budget of 1 the before shot is the whole budget, so no after
// screenshot may be taken.
func TestStateCov_AttachEvidencePayload_BudgetOneSkipsAfterShot(t *testing.T) {
	t.Setenv(evidenceRetryEnv, "0")
	t.Setenv(evidenceMaxCapturesEnv, "1")
	rec := statecovInstallCapture(t, func(int) evidenceShot { return evidenceShot{Path: "/before.png"} })
	h := NewInteractActionHandler(&Deps{})
	h.ArmEvidenceForCommand("c1", "click", json.RawMessage(`{"evidence":"always"}`), "cli")

	data := map[string]any{}
	h.AttachEvidencePayload("c1", data)

	ev, _ := data["evidence"].(map[string]any)
	if ev["before"] != "/before.png" {
		t.Errorf("before = %v, want /before.png", ev["before"])
	}
	if _, has := ev["after"]; has {
		t.Errorf("after must be absent with a capture budget of 1: %v", ev)
	}
	if rec.calls() != 1 {
		t.Errorf("capture called %d times, want 1", rec.calls())
	}
}

// ===========================================================================
// interact_retry_contract_state.go — arming, parent chaining, pruning
// ===========================================================================

func TestStateCov_ArmRetryContract_FirstAttemptHasNoParent(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})

	h.armRetryContract("c1", "CLICK ", json.RawMessage(`{"selector":"#go"}`))

	st, ok := h.getRetryState("c1")
	if !ok {
		t.Fatal("retry state was not stored")
	}
	if st.Attempt != 1 || st.MaxAttempts != maxRetryAttemptsPerStep {
		t.Errorf("attempt/max = %d/%d, want 1/%d", st.Attempt, st.MaxAttempts, maxRetryAttemptsPerStep)
	}
	if st.Action != "click" {
		t.Errorf("action = %q, want click (lowercased and trimmed)", st.Action)
	}
	if st.Strategy != "selector" {
		t.Errorf("strategy = %q, want selector", st.Strategy)
	}
	if !st.ChangedStrategy {
		t.Error("a first attempt must count as a changed strategy")
	}
	if st.PolicyViolation != "" || st.ParentCorrelationID != "" {
		t.Errorf("violation/parent = %q/%q, want empty", st.PolicyViolation, st.ParentCorrelationID)
	}
	if st.CreatedAt.IsZero() {
		t.Error("CreatedAt must be set — pruning orders by it")
	}
}

func TestStateCov_ArmRetryContract_DerivesActionFromArgsWhenBlank(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("c1", "", json.RawMessage(`{"what":"Fill_Form","element_id":"el-3"}`))

	st, _ := h.getRetryState("c1")
	if st.Action != "fill_form" {
		t.Errorf("action = %q, want fill_form", st.Action)
	}
	if st.Strategy != "element_handle" {
		t.Errorf("strategy = %q, want element_handle", st.Strategy)
	}
}

func TestStateCov_ArmRetryContract_ParentChainIncrementsAttempt(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("parent", "click", json.RawMessage(`{"selector":"#a"}`))
	h.armRetryContract("child", "click", json.RawMessage(`{"selector":"#b","correlation_id":"parent"}`))

	st, _ := h.getRetryState("child")
	if st.Attempt != 2 {
		t.Errorf("attempt = %d, want 2", st.Attempt)
	}
	if st.ParentCorrelationID != "parent" {
		t.Errorf("parent = %q, want parent", st.ParentCorrelationID)
	}
	if !st.ChangedStrategy {
		t.Error("a different selector must register as a changed strategy")
	}
	if st.PolicyViolation != "" {
		t.Errorf("violation = %q, want empty", st.PolicyViolation)
	}
}

// Retrying with byte-identical targeting is the policy violation the retry
// contract exists to catch.
func TestStateCov_ArmRetryContract_IdenticalStrategyIsAViolation(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("parent", "click", json.RawMessage(`{"selector":"#a"}`))
	h.armRetryContract("child", "click", json.RawMessage(`{"selector":"#a","correlation_id":"parent"}`))

	st, _ := h.getRetryState("child")
	if st.ChangedStrategy {
		t.Error("ChangedStrategy must be false when the fingerprint is unchanged")
	}
	if st.PolicyViolation != "strategy_unchanged" {
		t.Errorf("violation = %q, want strategy_unchanged", st.PolicyViolation)
	}
}

func TestStateCov_ArmRetryContract_ClampsAttemptsAtTheLimit(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.storeRetryState("parent", &commandRetryState{
		Attempt:             2,
		MaxAttempts:         maxRetryAttemptsPerStep,
		StrategyFingerprint: "different-fingerprint",
		CreatedAt:           time.Now(),
	})

	h.armRetryContract("child", "click", json.RawMessage(`{"selector":"#z","correlation_id":"parent"}`))

	st, _ := h.getRetryState("child")
	if st.Attempt != maxRetryAttemptsPerStep {
		t.Errorf("attempt = %d, want clamped to %d", st.Attempt, maxRetryAttemptsPerStep)
	}
	if st.PolicyViolation != "attempt_limit_exceeded" {
		t.Errorf("violation = %q, want attempt_limit_exceeded", st.PolicyViolation)
	}
}

// An expired parent must still be treated as a retry, not as a fresh attempt —
// otherwise a caller could loop forever by letting the context lapse.
func TestStateCov_ArmRetryContract_MissingParentStillCountsAsRetry(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("child", "click", json.RawMessage(`{"selector":"#a","correlation_id":"ghost"}`))

	st, _ := h.getRetryState("child")
	if st.Attempt != 2 {
		t.Errorf("attempt = %d, want 2", st.Attempt)
	}
	if st.PolicyViolation != "parent_context_missing" {
		t.Errorf("violation = %q, want parent_context_missing", st.PolicyViolation)
	}
	if !st.ChangedStrategy {
		t.Error("ChangedStrategy must stay true when the parent is unknown")
	}
}

func TestStateCov_ArmRetryContract_IgnoresEmptyCorrelationAndNilHandler(t *testing.T) {
	var nilHandler *InteractActionHandler
	nilHandler.armRetryContract("c1", "click", nil)

	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("", "click", json.RawMessage(`{"selector":"#a"}`))
	if _, ok := h.getRetryState(""); ok {
		t.Error("empty correlation id must not be stored")
	}
}

func TestStateCov_PruneRetryStates_EvictsTheOldestEntry(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	base := time.Now()
	h.retryByCommand["old"] = &commandRetryState{CreatedAt: base.Add(-2 * time.Hour)}
	h.retryByCommand["mid"] = &commandRetryState{CreatedAt: base.Add(-1 * time.Hour)}
	h.retryByCommand["new"] = &commandRetryState{CreatedAt: base}

	h.retryContractMu.Lock()
	h.pruneRetryStatesLocked(2)
	h.retryContractMu.Unlock()

	if _, ok := h.getRetryState("old"); ok {
		t.Error("oldest entry must be evicted")
	}
	if _, ok := h.getRetryState("mid"); !ok {
		t.Error("mid entry must survive")
	}
	if _, ok := h.getRetryState("new"); !ok {
		t.Error("newest entry must survive")
	}

	// Under the cap, pruning is a no-op.
	h.retryContractMu.Lock()
	h.pruneRetryStatesLocked(10)
	h.retryContractMu.Unlock()
	if len(h.retryByCommand) != 2 {
		t.Errorf("len = %d, want 2 (no eviction below the cap)", len(h.retryByCommand))
	}
}

// The retry map is unbounded input from callers; storeRetryState must cap it.
func TestStateCov_StoreRetryState_CapsMapGrowth(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	base := time.Now()
	for i := 0; i < 2100; i++ {
		h.storeRetryState(statecovID(i), &commandRetryState{CreatedAt: base.Add(time.Duration(i) * time.Millisecond)})
	}
	if got := len(h.retryByCommand); got != 2048 {
		t.Errorf("retry map size = %d, want 2048", got)
	}
	if _, ok := h.getRetryState(statecovID(0)); ok {
		t.Error("the oldest correlation id should have been evicted")
	}
	if _, ok := h.getRetryState(statecovID(2099)); !ok {
		t.Error("the newest correlation id must be retained")
	}
}

// statecovID builds a unique synthetic correlation ID.
func statecovID(i int) string { return "statecov-corr-" + strconv.Itoa(i) }

// ===========================================================================
// interact_retry_contract_response.go — retry_context decoration
// ===========================================================================

func TestStateCov_DeriveRetryReason_PrecedenceOrder(t *testing.T) {
	cases := []struct {
		name     string
		data     map[string]any
		fallback string
		want     string
	}{
		{"error_code wins", map[string]any{"error_code": "not_found", "error": "boom"}, "fb", "not_found"},
		{"error used when no code", map[string]any{"error": " boom "}, "fb", "boom"},
		{"blank code falls through", map[string]any{"error_code": "   ", "error": "boom"}, "fb", "boom"},
		{"fallback used when data empty", map[string]any{}, " fb ", "fb"},
		{"nil data uses fallback", nil, "fb", "fb"},
		{"unknown when nothing available", map[string]any{}, "   ", "unknown"},
		{"non-string code ignored", map[string]any{"error_code": 42}, "", "unknown"},
	}
	for _, tc := range cases {
		if got := deriveRetryReason(tc.data, tc.fallback); got != tc.want {
			t.Errorf("%s: deriveRetryReason = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestStateCov_AttachRetryContext_NoOpsWithoutState(t *testing.T) {
	var nilHandler *InteractActionHandler
	if d := nilHandler.AttachRetryContext("c1", map[string]any{}, "error", ""); d.Terminal {
		t.Error("nil handler must return a non-terminal decision")
	}

	h := NewInteractActionHandler(&Deps{})
	data := map[string]any{"status": "error"}
	h.AttachRetryContext("unarmed", data, "error", "boom")
	if _, has := data["retry_context"]; has {
		t.Errorf("unarmed correlation id must not be decorated: %v", data)
	}
	h.AttachRetryContext("", data, "error", "boom")
	h.AttachRetryContext("c1", nil, "error", "boom")
}

func TestStateCov_AttachRetryContext_SuccessIsNeverTerminal(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("c1", "click", json.RawMessage(`{"selector":"#a"}`))

	data := map[string]any{"status": "complete"}
	decision := h.AttachRetryContext("c1", data, "complete", "")

	if decision.Terminal {
		t.Error("a completed command must not be terminal")
	}
	rc, ok := data["retry_context"].(map[string]any)
	if !ok {
		t.Fatalf("retry_context missing: %v", data)
	}
	if rc["reason"] != "success" {
		t.Errorf("reason = %v, want success", rc["reason"])
	}
	if rc["attempt"] != 1 || rc["max_attempts"] != maxRetryAttemptsPerStep {
		t.Errorf("attempt/max = %v/%v, want 1/%d", rc["attempt"], rc["max_attempts"], maxRetryAttemptsPerStep)
	}
	if rc["terminal_stop"] != false {
		t.Errorf("terminal_stop = %v, want false", rc["terminal_stop"])
	}
	if _, has := data["retryable"]; has {
		t.Errorf("a success response must not be annotated as retryable: %v", data)
	}
	if _, has := data["evidence_summary"]; has {
		t.Error("a success response must not carry an evidence_summary")
	}
}

func TestStateCov_AttachRetryContext_FirstFailureAllowsOneRetry(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("c1", "click", json.RawMessage(`{"selector":"#a"}`))

	data := map[string]any{"error_code": "element_not_found"}
	decision := h.AttachRetryContext("c1", data, "error", "ignored-fallback")

	if decision.Terminal {
		t.Errorf("attempt 1 must not be terminal, got cause %q", decision.Cause)
	}
	if data["retryable"] != true {
		t.Errorf("retryable = %v, want true", data["retryable"])
	}
	statecovSub(t, data["retry"].(string), "changed strategy", "retry guidance")
	rc := data["retry_context"].(map[string]any)
	if rc["reason"] != "element_not_found" {
		t.Errorf("reason = %v, want element_not_found", rc["reason"])
	}
	if _, has := rc["terminal_cause"]; has {
		t.Errorf("terminal_cause must be absent for a non-terminal failure: %v", rc)
	}
	if _, has := data["evidence_summary"]; has {
		t.Error("evidence_summary is reserved for terminal failures")
	}
}

func TestStateCov_AttachRetryContext_SecondFailureIsTerminalWithEvidence(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("parent", "click", json.RawMessage(`{"selector":"#a"}`))
	h.armRetryContract("child", "click", json.RawMessage(`{"selector":"#b","correlation_id":"parent"}`))

	data := map[string]any{
		"error_code":    "timeout",
		"evidence":      map[string]any{"before": "/b.png"},
		"effective_url": "https://app.test/checkout",
		"resolved_url":  "https://ignored.test/",
	}
	decision := h.AttachRetryContext("child", data, "timeout", "")

	if !decision.Terminal || decision.Cause != "max_attempts_reached" {
		t.Fatalf("decision = %+v, want terminal/max_attempts_reached", decision)
	}
	if data["terminal"] != true || data["retryable"] != false {
		t.Errorf("terminal/retryable = %v/%v, want true/false", data["terminal"], data["retryable"])
	}
	statecovSub(t, data["retry"].(string), "Stop retrying", "terminal retry guidance")

	rc := data["retry_context"].(map[string]any)
	if rc["terminal_stop"] != true || rc["terminal_cause"] != "max_attempts_reached" {
		t.Errorf("retry_context terminal fields = %v/%v", rc["terminal_stop"], rc["terminal_cause"])
	}
	if rc["parent_correlation_id"] != "parent" {
		t.Errorf("parent_correlation_id = %v, want parent", rc["parent_correlation_id"])
	}

	summary, ok := data["evidence_summary"].(map[string]any)
	if !ok {
		t.Fatalf("evidence_summary missing: %v", data)
	}
	if summary["correlation_id"] != "child" || summary["failure_reason"] != "timeout" {
		t.Errorf("summary identity = %v/%v", summary["correlation_id"], summary["failure_reason"])
	}
	if summary["url"] != "https://app.test/checkout" {
		t.Errorf("url = %v, want effective_url to win over resolved_url", summary["url"])
	}
	captured, _ := summary["captured_evidence"].(map[string]any)
	if captured["before"] != "/b.png" {
		t.Errorf("captured_evidence = %v, want the evidence payload", summary["captured_evidence"])
	}
	required, ok := summary["required"].([]string)
	if !ok {
		t.Fatalf("required = %T, want []string", summary["required"])
	}
	want := []string{"command_result", "screenshot", "scoped_list_interactive_output"}
	if len(required) != len(want) {
		t.Fatalf("required = %v, want %v", required, want)
	}
	for i := range want {
		if required[i] != want[i] {
			t.Errorf("required[%d] = %q, want %q", i, required[i], want[i])
		}
	}
}

// Repeating the same strategy is terminal even on attempt 2 of 2, and the
// unchanged-strategy cause must win over the attempt-limit cause.
func TestStateCov_AttachRetryContext_UnchangedStrategyCauseWins(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("parent", "click", json.RawMessage(`{"selector":"#a"}`))
	h.armRetryContract("child", "click", json.RawMessage(`{"selector":"#a","correlation_id":"parent"}`))

	data := map[string]any{"error": "still_failing"}
	decision := h.AttachRetryContext("child", data, "cancelled", "")

	if !decision.Terminal || decision.Cause != "strategy_not_changed" {
		t.Fatalf("decision = %+v, want terminal/strategy_not_changed", decision)
	}
	rc := data["retry_context"].(map[string]any)
	if rc["policy_violation"] != "strategy_unchanged" {
		t.Errorf("policy_violation = %v, want strategy_unchanged", rc["policy_violation"])
	}
	if rc["changed_strategy"] != false {
		t.Errorf("changed_strategy = %v, want false", rc["changed_strategy"])
	}
}

// Handler-supplied recovery guidance must not be overwritten by the generic text.
func TestStateCov_AttachRetryContext_PreservesExistingRetryGuidance(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("c1", "click", json.RawMessage(`{"selector":"#a"}`))

	data := map[string]any{"error_code": "csp_blocked", "retry": "Use world=isolated", "retryable": false}
	h.AttachRetryContext("c1", data, "error", "")

	if data["retry"] != "Use world=isolated" {
		t.Errorf("retry = %v, want the handler's own guidance preserved", data["retry"])
	}
	if data["retryable"] != false {
		t.Errorf("retryable = %v, want the handler's own value preserved", data["retryable"])
	}
}

func TestStateCov_AttachRetryContext_ExpiredStatusCountsAsFailure(t *testing.T) {
	h := NewInteractActionHandler(&Deps{})
	h.armRetryContract("c1", "click", json.RawMessage(`{"selector":"#a"}`))

	data := map[string]any{}
	h.AttachRetryContext("c1", data, "expired", "extension_gone")

	if data["retryable"] != true {
		t.Errorf("retryable = %v, want true for an expired first attempt", data["retryable"])
	}
	rc := data["retry_context"].(map[string]any)
	if rc["reason"] != "extension_gone" {
		t.Errorf("reason = %v, want the fallback reason", rc["reason"])
	}
}

func TestStateCov_BuildRetryEvidenceSummary_FallsBackToResolvedURL(t *testing.T) {
	summary := buildRetryEvidenceSummary("c1", "timeout", nil, map[string]any{
		"resolved_url": "https://fallback.test/",
	})
	if summary["url"] != "https://fallback.test/" {
		t.Errorf("url = %v, want the resolved_url fallback", summary["url"])
	}
	if _, has := summary["retry_context"]; has {
		t.Error("retry_context must be omitted when nil")
	}
	if _, has := summary["captured_evidence"]; has {
		t.Error("captured_evidence must be omitted when no evidence exists")
	}

	bare := buildRetryEvidenceSummary("c2", "boom", nil, nil)
	if _, has := bare["url"]; has {
		t.Errorf("url must be omitted with no response data: %v", bare)
	}
	if bare["next_action"] == "" {
		t.Error("next_action must always be present")
	}

	blank := buildRetryEvidenceSummary("c3", "boom", nil, map[string]any{"effective_url": "   "})
	if _, has := blank["url"]; has {
		t.Errorf("a whitespace-only effective_url must not be reported: %v", blank["url"])
	}
}

// ===========================================================================
// interact_state_list_delete.go — snapshot name resolution + CRUD
// ===========================================================================

func TestStateCov_ResolveStateSnapshotName_PrefersCanonicalOverLegacy(t *testing.T) {
	cases := []struct{ snapshot, legacy, want string }{
		{"canonical", "legacy", "canonical"},
		{"", "legacy", "legacy"},
		{"canonical", "", "canonical"},
		{"", "", ""},
	}
	for _, tc := range cases {
		if got := resolveStateSnapshotName(tc.snapshot, tc.legacy); got != tc.want {
			t.Errorf("resolveStateSnapshotName(%q,%q) = %q, want %q", tc.snapshot, tc.legacy, got, tc.want)
		}
	}
}

func TestStateCov_StateList_EmptyStoreReportsZero(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovOK(t, hs.state.HandleStateList(statecovReq(), nil))
	if payload["count"] != float64(0) {
		t.Errorf("count = %v, want 0", payload["count"])
	}
	states, ok := payload["states"].([]any)
	if !ok || len(states) != 0 {
		t.Errorf("states = %v, want an empty array (never null)", payload["states"])
	}
}

func TestStateCov_StateList_ReturnsMetadataForEachSnapshot(t *testing.T) {
	hs := statecovNewHarness(t)
	if err := hs.store.Save(act.StateNamespace, "checkout",
		[]byte(`{"url":"https://shop.test/cart","title":"Cart","saved_at":"2024-05-01T10:00:00Z","form_values":{"qty":"2"}}`)); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateList(statecovReq(), nil))
	if payload["count"] != float64(1) {
		t.Fatalf("count = %v, want 1", payload["count"])
	}
	entry := payload["states"].([]any)[0].(map[string]any)
	if entry["name"] != "checkout" || entry["url"] != "https://shop.test/cart" ||
		entry["title"] != "Cart" || entry["saved_at"] != "2024-05-01T10:00:00Z" {
		t.Errorf("entry = %v, want name/url/title/saved_at from the stored snapshot", entry)
	}
	// The listing is metadata only — bulky captured state must not be inlined.
	if _, has := entry["form_values"]; has {
		t.Errorf("list entries must not inline captured state: %v", entry)
	}
}

// A snapshot file that is not valid JSON must degrade to a name-only entry
// rather than failing the whole listing.
func TestStateCov_StateList_CorruptSnapshotDegradesToNameOnly(t *testing.T) {
	hs := statecovNewHarness(t)
	if err := hs.store.Save(act.StateNamespace, "good", []byte(`{"url":"https://ok.test/","title":"OK"}`)); err != nil {
		t.Fatalf("seed good: %v", err)
	}
	if err := hs.store.Save(act.StateNamespace, "corrupt", []byte(`{"url": `)); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateList(statecovReq(), nil))
	if payload["count"] != float64(2) {
		t.Fatalf("count = %v, want 2", payload["count"])
	}
	for _, raw := range payload["states"].([]any) {
		entry := raw.(map[string]any)
		if entry["name"] == "corrupt" {
			if len(entry) != 1 {
				t.Errorf("corrupt entry = %v, want only the name key", entry)
			}
		}
		if entry["name"] == "good" && entry["url"] != "https://ok.test/" {
			t.Errorf("good entry lost its metadata: %v", entry)
		}
	}
}

// Non-string metadata must be dropped, not coerced into the response.
func TestStateCov_BuildStateEntry_SkipsMissingKeysAndNonStringFields(t *testing.T) {
	hs := statecovNewHarness(t)

	missing := hs.state.buildStateEntry("does-not-exist")
	if len(missing) != 1 || missing["name"] != "does-not-exist" {
		t.Errorf("entry for a missing key = %v, want only the name", missing)
	}

	if err := hs.store.Save(act.StateNamespace, "typed", []byte(`{"url":123,"title":"T"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	entry := hs.state.buildStateEntry("typed")
	if _, has := entry["url"]; has {
		t.Errorf("numeric url must be dropped: %v", entry)
	}
	if entry["title"] != "T" {
		t.Errorf("title = %v, want T", entry["title"])
	}
}

func TestStateCov_StateList_BlockedWhenSessionStoreUnavailable(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.blockSessionStore = true

	payload := statecovFail(t, hs.state.HandleStateList(statecovReq(), nil))
	if got := statecovStr(t, payload, "error_code"); got != ErrNotInitialized {
		t.Errorf("error_code = %q, want %q", got, ErrNotInitialized)
	}
}

func TestStateCov_StateDelete_RemovesSnapshotAndRecordsAction(t *testing.T) {
	hs := statecovNewHarness(t)
	if err := hs.store.Save(act.StateNamespace, "temp", []byte(`{"url":"https://a.test/"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateDelete(statecovReq(), json.RawMessage(`{"snapshot_name":"temp"}`)))
	if payload["status"] != "deleted" || payload["snapshot_name"] != "temp" {
		t.Errorf("payload = %v, want deleted/temp", payload)
	}

	if _, err := hs.store.Load(act.StateNamespace, "temp"); err == nil {
		t.Error("snapshot still loadable after delete")
	}

	recs := hs.statecovRecords()
	if len(recs) != 1 || recs[0].Action != "delete_state" || recs[0].Extra["snapshot_name"] != "temp" {
		t.Errorf("recorded actions = %+v, want a single delete_state for temp", recs)
	}
}

func TestStateCov_StateDelete_MissingSnapshotIsNoData(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.state.HandleStateDelete(statecovReq(), json.RawMessage(`{"name":"ghost"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrNoData {
		t.Errorf("error_code = %q, want %q", got, ErrNoData)
	}
	statecovSub(t, statecovStr(t, payload, "message"), "ghost", "message names the snapshot")
	statecovSub(t, statecovStr(t, payload, "recovery_playbook"), "list_states", "recovery points at list_states")
	if got := statecovStr(t, payload, "hint"); got != "statecov-diag" {
		t.Errorf("hint = %q, want the injected diagnostic hint", got)
	}
	if n := len(hs.statecovRecords()); n != 0 {
		t.Errorf("recorded %d actions for a failed delete, want 0", n)
	}
}

func TestStateCov_StateDelete_RequiresASnapshotName(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.state.HandleStateDelete(statecovReq(), json.RawMessage(`{}`)))
	if got := statecovStr(t, payload, "param"); got != "snapshot_name" {
		t.Errorf("param = %q, want snapshot_name", got)
	}
	statecovSub(t, statecovStr(t, payload, "recovery_playbook"), "name", "playbook mentions the legacy alias")

	bad := statecovFail(t, hs.state.HandleStateDelete(statecovReq(), json.RawMessage(`{`)))
	if got := statecovStr(t, bad, "error_code"); got != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", got, ErrInvalidJSON)
	}
}

// ===========================================================================
// interact_state_capture.go — browser capture / restore / navigation queueing
// ===========================================================================

func TestStateCov_CaptureState_ReportsPilotAndConnectionGates(t *testing.T) {
	hs := statecovNewHarness(t)

	hs.cap.SetPilotEnabled(false)
	if got := hs.state.CaptureState(statecovReq()); got.Status != act.StateCaptureStatusPilotDisabled || got.Data != nil {
		t.Errorf("pilot off => %+v, want %s with nil data", got, act.StateCaptureStatusPilotDisabled)
	}

	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionDisconnectForTest()
	if got := hs.state.CaptureState(statecovReq()); got.Status != act.StateCaptureStatusExtensionDisconnected {
		t.Errorf("extension down => %+v, want %s", got, act.StateCaptureStatusExtensionDisconnected)
	}

	if n := len(hs.statecovQueries()); n != 0 {
		t.Errorf("enqueued %d queries while gated, want 0", n)
	}
}

func TestStateCov_CaptureState_EnqueueFailureIsAnError(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	hs.blockEnqueue = true

	got := hs.state.CaptureState(statecovReq())
	if got.Status != act.StateCaptureStatusError {
		t.Errorf("status = %q, want %q", got.Status, act.StateCaptureStatusError)
	}
	q := hs.statecovLastQuery()
	if q.Type != "execute" || !strings.HasPrefix(q.CorrelationID, "state_capture_") {
		t.Errorf("query = %+v, want an execute query with a state_capture_ correlation id", q)
	}
	params := hs.statecovLastParams()
	if params["action"] != "execute_js" || params["world"] != "main" {
		t.Errorf("params = %v, want execute_js in the main world", params)
	}
	if params["script"] != act.StateCaptureScript {
		t.Error("capture must run the canonical StateCaptureScript")
	}
}

// A command that never resolves must report a timeout, not a hang or an error.
func TestStateCov_CaptureState_UnresolvedCommandTimesOut(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	// No autoComplete: the correlation ID is never registered, so WaitForCommand
	// returns "not found" immediately.

	if got := hs.state.CaptureState(statecovReq()); got.Status != act.StateCaptureStatusTimeout {
		t.Errorf("status = %q, want %q", got.Status, act.StateCaptureStatusTimeout)
	}
}

func TestStateCov_CaptureState_ExtensionErrorsAndBadPayloads(t *testing.T) {
	cases := []struct {
		name   string
		status string
		result string
		errMsg string
		want   string
	}{
		{"extension reported an error", "complete", `{"success":true,"result":{"form_values":{}}}`, "boom", act.StateCaptureStatusError},
		{"non-complete terminal status", "expired", `{}`, "", act.StateCaptureStatusError},
		{"empty result body", "complete", ``, "", act.StateCaptureStatusError},
		{"payload missing known fields", "complete", `{"nothing":"useful"}`, "", act.StateCaptureStatusError},
		{"script reported failure", "complete", `{"success":false,"message":"blocked by CSP"}`, "", act.StateCaptureStatusError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := statecovNewHarness(t)
			hs.cap.SetPilotEnabled(true)
			hs.cap.SimulateExtensionConnectForTest()
			hs.autoComplete = func(queries.PendingQuery) (string, json.RawMessage, string) {
				return tc.status, json.RawMessage(tc.result), tc.errMsg
			}

			got := hs.state.CaptureState(statecovReq())
			if got.Status != tc.want {
				t.Errorf("status = %q, want %q", got.Status, tc.want)
			}
			if got.Data != nil {
				t.Errorf("data = %v, want nil on failure", got.Data)
			}
		})
	}
}

func TestStateCov_CaptureState_ParsesSuccessEnvelope(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	hs.autoComplete = func(queries.PendingQuery) (string, json.RawMessage, string) {
		return "complete", json.RawMessage(`{"success":true,"result":{"form_values":{"email":"a@b.test"},"scroll_position":{"y":120}}}`), ""
	}

	got := hs.state.CaptureState(statecovReq())
	if got.Status != act.StateCaptureStatusCaptured {
		t.Fatalf("status = %q, want %q", got.Status, act.StateCaptureStatusCaptured)
	}
	forms, _ := got.Data["form_values"].(map[string]any)
	if forms["email"] != "a@b.test" {
		t.Errorf("form_values = %v, want email=a@b.test", got.Data["form_values"])
	}
	scroll, _ := got.Data["scroll_position"].(map[string]any)
	if scroll["y"] != float64(120) {
		t.Errorf("scroll_position = %v, want y=120", got.Data["scroll_position"])
	}
}

func TestStateCov_QueueStateRestore_ReturnsEmptyWhenEnqueueBlocked(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.blockEnqueue = true

	got := hs.state.queueStateRestore(statecovReq(), map[string]any{"a": "1"}, nil, nil, nil, nil)
	if got != "" {
		t.Errorf("correlation id = %q, want empty when the queue rejects the command", got)
	}
}

func TestStateCov_QueueStateRestore_EmbedsRestoreDataInScript(t *testing.T) {
	hs := statecovNewHarness(t)

	corr := hs.state.queueStateRestore(statecovReq(),
		map[string]any{"email": "a@b.test"}, map[string]any{"y": 40},
		map[string]any{"theme": "dark"}, map[string]any{"tmp": "1"}, map[string]any{"sid": "xyz"})

	if !strings.HasPrefix(corr, "state_restore_") {
		t.Errorf("correlation id = %q, want a state_restore_ prefix", corr)
	}
	q := hs.statecovLastQuery()
	if q.Type != "execute" || q.CorrelationID != corr {
		t.Errorf("query = %+v, want an execute query carrying %q", q, corr)
	}
	script, _ := hs.statecovLastParams()["script"].(string)
	for _, want := range []string{"a@b.test", "theme", "dark", "tmp", "sid", "xyz"} {
		statecovSub(t, script, want, "restore script payload")
	}
}

func TestStateCov_QueueStateNavigation_SkipsWithoutURLOrPermission(t *testing.T) {
	cases := []struct {
		name      string
		state     map[string]any
		pilot     bool
		connected bool
	}{
		{"no url key", map[string]any{}, true, true},
		{"empty url", map[string]any{"url": ""}, true, true},
		{"non-string url", map[string]any{"url": 7}, true, true},
		{"pilot disabled", map[string]any{"url": "https://a.test/"}, false, true},
		{"extension down", map[string]any{"url": "https://a.test/"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := statecovNewHarness(t)
			hs.cap.SetPilotEnabled(tc.pilot)
			if tc.connected {
				hs.cap.SimulateExtensionConnectForTest()
			} else {
				hs.cap.SimulateExtensionDisconnectForTest()
			}

			hs.state.QueueStateNavigation(statecovReq(), tc.state)

			if _, has := tc.state["navigation_queued"]; has {
				t.Errorf("state = %v, want no navigation_queued marker", tc.state)
			}
			if n := len(hs.statecovQueries()); n != 0 {
				t.Errorf("enqueued %d navigation queries, want 0", n)
			}
		})
	}
}

func TestStateCov_QueueStateNavigation_MarksStateOnSuccess(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()

	stateData := map[string]any{"url": "https://app.test/dashboard"}
	hs.state.QueueStateNavigation(statecovReq(), stateData)

	if stateData["navigation_queued"] != true {
		t.Errorf("navigation_queued = %v, want true", stateData["navigation_queued"])
	}
	corr, _ := stateData["correlation_id"].(string)
	if !strings.HasPrefix(corr, "nav_") {
		t.Errorf("correlation_id = %q, want a nav_ prefix", corr)
	}
	q := hs.statecovLastQuery()
	if q.Type != "browser_action" || q.CorrelationID != corr {
		t.Errorf("query = %+v, want a browser_action carrying %q", q, corr)
	}
	params := hs.statecovLastParams()
	if params["action"] != "navigate" || params["url"] != "https://app.test/dashboard" {
		t.Errorf("params = %v, want navigate to the saved url", params)
	}
}

// A rejected enqueue must leave the state map unmarked, or load_state would
// claim a navigation that never happened.
func TestStateCov_QueueStateNavigation_LeavesStateCleanWhenEnqueueBlocked(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	hs.blockEnqueue = true

	stateData := map[string]any{"url": "https://app.test/"}
	hs.state.QueueStateNavigation(statecovReq(), stateData)

	if _, has := stateData["navigation_queued"]; has {
		t.Errorf("state = %v, want no navigation_queued marker after a blocked enqueue", stateData)
	}
}

// ===========================================================================
// interact_state_save_load.go — save/load handlers and round-trip fidelity
// ===========================================================================

// statecovRedactor replaces any string value containing "token" — a stand-in for
// the real redaction engine.
type statecovRedactor struct{ calls int }

func (r *statecovRedactor) RedactMapValues(m map[string]any) map[string]any {
	r.calls++
	out := make(map[string]any, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok && strings.Contains(s, "token") {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v
	}
	return out
}

func TestStateCov_StateSave_RequiresASnapshotNameAndValidJSON(t *testing.T) {
	hs := statecovNewHarness(t)

	missing := statecovFail(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{}`)))
	if got := statecovStr(t, missing, "param"); got != "snapshot_name" {
		t.Errorf("param = %q, want snapshot_name", got)
	}

	bad := statecovFail(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{"snapshot_name":`)))
	if got := statecovStr(t, bad, "error_code"); got != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", got, ErrInvalidJSON)
	}

	keys, err := hs.store.List(act.StateNamespace)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("wrote %v despite rejecting the request", keys)
	}
}

func TestStateCov_StateSave_BlockedWhenSessionStoreUnavailable(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.blockSessionStore = true

	payload := statecovFail(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{"snapshot_name":"s1"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrNotInitialized {
		t.Errorf("error_code = %q, want %q", got, ErrNotInitialized)
	}
	if n := len(hs.statecovQueries()); n != 0 {
		t.Errorf("enqueued %d capture queries before the store check, want 0", n)
	}
}

// The legacy "name" alias must keep working for older clients.
func TestStateCov_StateSave_AcceptsLegacyNameAlias(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.UpdateTrackedTab(4, "https://legacy.test/page", "Legacy")

	payload := statecovOK(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{"name":"legacy-snap"}`)))
	if payload["snapshot_name"] != "legacy-snap" {
		t.Errorf("snapshot_name = %v, want legacy-snap", payload["snapshot_name"])
	}
	if _, err := hs.store.Load(act.StateNamespace, "legacy-snap"); err != nil {
		t.Errorf("snapshot not persisted under the legacy name: %v", err)
	}
}

// Save→load must preserve the tracked-tab identity exactly.
func TestStateCov_StateSaveLoad_RoundTripsTabIdentity(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionDisconnectForTest()
	hs.cap.UpdateTrackedTab(17, "https://app.test/orders?page=2", "Orders — page 2")

	saveResp := statecovOK(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{"snapshot_name":"orders"}`)))
	if saveResp["status"] != "saved" {
		t.Errorf("status = %v, want saved", saveResp["status"])
	}
	if saveResp["state_capture"] != act.StateCaptureStatusExtensionDisconnected {
		t.Errorf("state_capture = %v, want %s", saveResp["state_capture"], act.StateCaptureStatusExtensionDisconnected)
	}
	savedState, _ := saveResp["state"].(map[string]any)
	if savedState["url"] != "https://app.test/orders?page=2" || savedState["title"] != "Orders — page 2" {
		t.Errorf("save response state = %v, want the tracked tab url/title", savedState)
	}

	loadResp := statecovOK(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"orders"}`)))
	if loadResp["status"] != "loaded" || loadResp["snapshot_name"] != "orders" {
		t.Errorf("load payload = %v, want loaded/orders", loadResp)
	}
	loaded, _ := loadResp["state"].(map[string]any)
	if loaded["url"] != "https://app.test/orders?page=2" {
		t.Errorf("url = %v, want the saved url", loaded["url"])
	}
	if loaded["title"] != "Orders — page 2" {
		t.Errorf("title = %v, want the saved title", loaded["title"])
	}
	if loaded["tab_id"] != float64(17) {
		t.Errorf("tab_id = %v, want 17", loaded["tab_id"])
	}
	savedAt, _ := loaded["saved_at"].(string)
	if _, err := time.Parse(time.RFC3339, savedAt); err != nil {
		t.Errorf("saved_at = %q, want RFC3339: %v", savedAt, err)
	}
	// Nothing restorable was captured, so restore must be explicitly skipped.
	if loadResp["state_restore"] != act.StateRestoreStatusNoData {
		t.Errorf("state_restore = %v, want %s", loadResp["state_restore"], act.StateRestoreStatusNoData)
	}
	if _, has := loadResp["restore_correlation_id"]; has {
		t.Error("restore_correlation_id must be absent when nothing is restored")
	}

	recs := hs.statecovRecords()
	if len(recs) != 2 || recs[0].Action != "save_state" || recs[1].Action != "load_state" {
		t.Fatalf("recorded actions = %+v, want save_state then load_state", recs)
	}
	if recs[0].URL != "https://app.test/orders?page=2" {
		t.Errorf("save_state recorded url = %q, want the tracked url", recs[0].URL)
	}
}

// Only the whitelisted StateDataFields are persisted; anything else the capture
// script returns must be dropped.
func TestStateCov_StateSave_PersistsOnlyWhitelistedCaptureFields(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	hs.cap.UpdateTrackedTab(3, "https://app.test/form", "Form")
	hs.autoComplete = func(queries.PendingQuery) (string, json.RawMessage, string) {
		return "complete", json.RawMessage(`{"success":true,"result":{
			"form_values":{"email":"a@b.test"},
			"scroll_position":{"y":90},
			"local_storage":{"theme":"dark"},
			"session_storage":{"tmp":"1"},
			"cookies":{"sid":"xyz"},
			"debug_noise":{"huge":"payload"}
		}}`), ""
	}

	saveResp := statecovOK(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{"snapshot_name":"form"}`)))
	if saveResp["state_capture"] != act.StateCaptureStatusCaptured {
		t.Fatalf("state_capture = %v, want captured", saveResp["state_capture"])
	}

	raw, err := hs.store.Load(act.StateNamespace, "form")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("parse persisted snapshot: %v", err)
	}
	for _, field := range act.StateDataFields {
		if _, has := persisted[field]; !has {
			t.Errorf("persisted snapshot is missing %q: %v", field, persisted)
		}
	}
	if _, has := persisted["debug_noise"]; has {
		t.Errorf("non-whitelisted capture field leaked into the snapshot: %v", persisted)
	}
	forms, _ := persisted["form_values"].(map[string]any)
	if forms["email"] != "a@b.test" {
		t.Errorf("form_values = %v, want email=a@b.test", persisted["form_values"])
	}
}

// #132: sensitive values must be scrubbed before they reach disk, not on read.
func TestStateCov_StateSave_RedactsBeforeWritingToDisk(t *testing.T) {
	hs := statecovNewHarness(t)
	red := &statecovRedactor{}
	hs.redaction = red
	hs.cap.UpdateTrackedTab(2, "https://app.test/?token=supersecret", "Home")

	statecovOK(t, hs.state.HandleStateSave(statecovReq(), json.RawMessage(`{"snapshot_name":"redacted"}`)))

	if red.calls != 1 {
		t.Errorf("redaction engine called %d times, want 1", red.calls)
	}
	raw, err := hs.store.Load(act.StateNamespace, "redacted")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if strings.Contains(string(raw), "supersecret") {
		t.Errorf("unredacted secret persisted to disk: %s", raw)
	}
	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if persisted["url"] != "[REDACTED]" {
		t.Errorf("persisted url = %v, want [REDACTED]", persisted["url"])
	}
}

// Path separators / traversal in a snapshot name must be rejected by the store
// rather than escaping the project directory.
func TestStateCov_StateSave_RejectsTraversalSnapshotNames(t *testing.T) {
	hs := statecovNewHarness(t)

	for _, name := range []string{"../escape", "nested/child"} {
		args, err := json.Marshal(map[string]string{"snapshot_name": name})
		if err != nil {
			t.Fatalf("marshal args: %v", err)
		}
		payload := statecovFail(t, hs.state.HandleStateSave(statecovReq(), args))
		if got := statecovStr(t, payload, "error_code"); got != ErrInternal {
			t.Errorf("%q: error_code = %q, want %q", name, got, ErrInternal)
		}
		statecovSub(t, statecovStr(t, payload, "message"), "Failed to save state", "save failure message")
	}
	if n := len(hs.statecovRecords()); n != 0 {
		t.Errorf("recorded %d actions for rejected saves, want 0", n)
	}
}

func TestStateCov_StateLoad_MissingSnapshotIsNoData(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"ghost"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrNoData {
		t.Errorf("error_code = %q, want %q", got, ErrNoData)
	}
	statecovSub(t, statecovStr(t, payload, "message"), "ghost", "message names the snapshot")
	if got := statecovStr(t, payload, "hint"); got != "statecov-diag" {
		t.Errorf("hint = %q, want the injected diagnostic hint", got)
	}
}

// A snapshot file corrupted on disk must produce a clear error, not a panic or
// a half-populated state.
func TestStateCov_StateLoad_CorruptSnapshotFileIsAnInternalError(t *testing.T) {
	hs := statecovNewHarness(t)
	if err := hs.store.Save(act.StateNamespace, "broken", []byte(`{"url":"https://a.test/"`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovFail(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"broken"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrInternal {
		t.Errorf("error_code = %q, want %q", got, ErrInternal)
	}
	statecovSub(t, statecovStr(t, payload, "recovery_playbook"), "corrupted", "playbook explains corruption")
	if n := len(hs.statecovRecords()); n != 0 {
		t.Errorf("recorded %d actions for a corrupt snapshot, want 0", n)
	}
}

func TestStateCov_StateLoad_RequiresASnapshotName(t *testing.T) {
	hs := statecovNewHarness(t)

	payload := statecovFail(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{}`)))
	if got := statecovStr(t, payload, "param"); got != "snapshot_name" {
		t.Errorf("param = %q, want snapshot_name", got)
	}

	bad := statecovFail(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"x":`)))
	if got := statecovStr(t, bad, "error_code"); got != ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", got, ErrInvalidJSON)
	}

	hs.blockSessionStore = true
	blocked := statecovFail(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"x"}`)))
	if got := statecovStr(t, blocked, "error_code"); got != ErrNotInitialized {
		t.Errorf("error_code = %q, want %q", got, ErrNotInitialized)
	}
}

func TestStateCov_StateLoad_QueuesRestoreWhenStateHasData(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	if err := hs.store.Save(act.StateNamespace, "rich", []byte(`{"url":"https://app.test/","form_values":{"email":"a@b.test"},"scroll_position":{"y":10}}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"rich"}`)))

	if payload["state_restore"] != act.StateRestoreStatusQueued {
		t.Fatalf("state_restore = %v, want %s", payload["state_restore"], act.StateRestoreStatusQueued)
	}
	corr, _ := payload["restore_correlation_id"].(string)
	if !strings.HasPrefix(corr, "state_restore_") {
		t.Errorf("restore_correlation_id = %q, want a state_restore_ prefix", corr)
	}
	q := hs.statecovLastQuery()
	if q.CorrelationID != corr || q.Type != "execute" {
		t.Errorf("query = %+v, want an execute query carrying %q", q, corr)
	}
	statecovSub(t, hs.statecovLastParams()["script"].(string), "a@b.test", "restore script")

	// include_url defaults to false, so no navigation may be queued.
	loaded, _ := payload["state"].(map[string]any)
	if _, has := loaded["navigation_queued"]; has {
		t.Errorf("navigation must not be queued without include_url: %v", loaded)
	}
}

func TestStateCov_StateLoad_IncludeURLQueuesNavigationBeforeRestore(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	if err := hs.store.Save(act.StateNamespace, "nav", []byte(`{"url":"https://app.test/deep","cookies":{"sid":"1"}}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateLoad(statecovReq(),
		json.RawMessage(`{"snapshot_name":"nav","include_url":true}`)))

	loaded, _ := payload["state"].(map[string]any)
	if loaded["navigation_queued"] != true {
		t.Errorf("navigation_queued = %v, want true", loaded["navigation_queued"])
	}

	all := hs.statecovQueries()
	if len(all) != 2 {
		t.Fatalf("enqueued %d queries, want 2 (navigate then restore)", len(all))
	}
	if all[0].Type != "browser_action" {
		t.Errorf("first query type = %q, want browser_action", all[0].Type)
	}
	if all[1].Type != "execute" {
		t.Errorf("second query type = %q, want execute", all[1].Type)
	}
}

func TestStateCov_StateLoad_RestoreGatesReportWhyItWasSkipped(t *testing.T) {
	cases := []struct {
		name      string
		pilot     bool
		connected bool
		want      string
	}{
		{"pilot disabled", false, true, act.StateRestoreStatusPilotDisabled},
		{"extension down", true, false, act.StateRestoreStatusExtensionDown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := statecovNewHarness(t)
			hs.cap.SetPilotEnabled(tc.pilot)
			if tc.connected {
				hs.cap.SimulateExtensionConnectForTest()
			} else {
				hs.cap.SimulateExtensionDisconnectForTest()
			}
			if err := hs.store.Save(act.StateNamespace, "s", []byte(`{"url":"https://a.test/","local_storage":{"k":"v"}}`)); err != nil {
				t.Fatalf("seed: %v", err)
			}

			payload := statecovOK(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"s"}`)))
			if payload["state_restore"] != tc.want {
				t.Errorf("state_restore = %v, want %s", payload["state_restore"], tc.want)
			}
			if _, has := payload["restore_correlation_id"]; has {
				t.Error("no restore correlation id may be reported when restore is skipped")
			}
			if n := len(hs.statecovQueries()); n != 0 {
				t.Errorf("enqueued %d queries while restore was gated, want 0", n)
			}
		})
	}
}

// Empty maps are not data: a snapshot whose every restorable container is empty
// must report no_data rather than queueing a pointless restore.
func TestStateCov_StateLoad_EmptyContainersCountAsNoData(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	if err := hs.store.Save(act.StateNamespace, "empty",
		[]byte(`{"url":"https://a.test/","form_values":{},"local_storage":{},"session_storage":{},"cookies":{},"scroll_position":{}}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"empty"}`)))
	if payload["state_restore"] != act.StateRestoreStatusNoData {
		t.Errorf("state_restore = %v, want %s (every container is empty)",
			payload["state_restore"], act.StateRestoreStatusNoData)
	}
	if n := len(hs.statecovQueries()); n != 0 {
		t.Errorf("enqueued %d queries, want 0", n)
	}
}

// A snapshot carrying only a scroll position is restorable data: scroll_position
// is captured, persisted, and replayed like the other fields, so it must queue a
// restore rather than report no_data (#9).
func TestStateCov_StateLoad_ScrollPositionAloneCountsAsData(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	if err := hs.store.Save(act.StateNamespace, "scroll",
		[]byte(`{"url":"https://a.test/","scroll_position":{"y":420}}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"scroll"}`)))
	if payload["state_restore"] != act.StateRestoreStatusQueued {
		t.Fatalf("state_restore = %v, want %s (scroll position is restorable data)",
			payload["state_restore"], act.StateRestoreStatusQueued)
	}
	corr, _ := payload["restore_correlation_id"].(string)
	if !strings.HasPrefix(corr, "state_restore_") {
		t.Errorf("restore_correlation_id = %q, want a state_restore_ prefix", corr)
	}
	// The restore script must actually carry the scroll position.
	statecovSub(t, hs.statecovLastParams()["script"].(string), "420", "restore script scroll value")
}

// A rejected restore enqueue must be reported honestly: the caller is told the
// restore did NOT start, with a reason, rather than being handed a "queued"
// status and an empty correlation id that points at nothing (#8).
func TestStateCov_StateLoad_BlockedRestoreEnqueueIsReportedHonestly(t *testing.T) {
	hs := statecovNewHarness(t)
	hs.cap.SetPilotEnabled(true)
	hs.cap.SimulateExtensionConnectForTest()
	hs.blockEnqueue = true
	if err := hs.store.Save(act.StateNamespace, "s", []byte(`{"url":"https://a.test/","cookies":{"sid":"1"}}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	payload := statecovOK(t, hs.state.HandleStateLoad(statecovReq(), json.RawMessage(`{"snapshot_name":"s"}`)))
	if payload["state_restore"] != act.StateRestoreStatusEnqueueRejected {
		t.Fatalf("state_restore = %v, want %s (a rejected enqueue is not 'queued')",
			payload["state_restore"], act.StateRestoreStatusEnqueueRejected)
	}
	if _, has := payload["restore_correlation_id"]; has {
		t.Error("no restore_correlation_id may be reported when the enqueue was rejected — there is no command to correlate")
	}
	detail, _ := payload["restore_detail"].(string)
	if !strings.Contains(detail, "not restored") {
		t.Errorf("restore_detail = %q, want a reason stating the state was not restored", detail)
	}
}

// ===========================================================================
// Remaining branches
// ===========================================================================

func TestStateCov_StoreRetryState_InitializesNilMap(t *testing.T) {
	h := &InteractActionHandler{}
	h.storeRetryState("c1", &commandRetryState{Attempt: 1, CreatedAt: time.Now()})
	if st, ok := h.getRetryState("c1"); !ok || st.Attempt != 1 {
		t.Error("storeRetryState must lazily create retryByCommand")
	}
}

// pruneRetryStatesLocked must trim the map all the way down to maxEntries in one
// call, evicting oldest-first — not remove a single entry and leave the map over
// the cap. It runs on every store today (so overflow is only ever one), but the
// helper is now correct for any overflow (#10).
func TestStateCov_PruneRetryStates_TrimsAllTheWayToTheCap(t *testing.T) {
	h := &InteractActionHandler{retryByCommand: map[string]*commandRetryState{}}
	base := time.Now()
	// 10 entries; higher index == created later.
	for i := 0; i < 10; i++ {
		h.retryByCommand["c"+strconv.Itoa(i)] = &commandRetryState{
			Attempt:   1,
			CreatedAt: base.Add(time.Duration(i) * time.Millisecond),
		}
	}

	h.retryContractMu.Lock()
	h.pruneRetryStatesLocked(3)
	h.retryContractMu.Unlock()

	if got := len(h.retryByCommand); got != 3 {
		t.Fatalf("map size after prune = %d, want 3 (must trim to the cap in one call)", got)
	}
	// The three newest survive; the oldest are evicted.
	for _, key := range []string{"c7", "c8", "c9"} {
		if _, ok := h.retryByCommand[key]; !ok {
			t.Errorf("newest entry %q was evicted, want kept", key)
		}
	}
	for _, key := range []string{"c0", "c6"} {
		if _, ok := h.retryByCommand[key]; ok {
			t.Errorf("oldest entry %q survived, want evicted", key)
		}
	}
}

func TestStateCov_DeleteCookie_HonoursExplicitPath(t *testing.T) {
	hs := statecovNewHarness(t)

	statecovOK(t, hs.action.HandleDeleteCookie(statecovReq(), json.RawMessage(`{"name":"sid","path":"/admin"}`)))

	script, _ := hs.statecovLastParams()["script"].(string)
	statecovSub(t, script, `"sid=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/admin"`, "explicit cookie path")
	if strings.Contains(script, "path=/;") {
		t.Errorf("default path must not be appended alongside an explicit one: %q", script)
	}
}

func TestStateCov_StateDelete_BlockedWhenSessionStoreUnavailable(t *testing.T) {
	hs := statecovNewHarness(t)
	if err := hs.store.Save(act.StateNamespace, "keep", []byte(`{"url":"https://a.test/"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	hs.blockSessionStore = true

	payload := statecovFail(t, hs.state.HandleStateDelete(statecovReq(), json.RawMessage(`{"snapshot_name":"keep"}`)))
	if got := statecovStr(t, payload, "error_code"); got != ErrNotInitialized {
		t.Errorf("error_code = %q, want %q", got, ErrNotInitialized)
	}
	if _, err := hs.store.Load(act.StateNamespace, "keep"); err != nil {
		t.Errorf("snapshot was deleted despite the store guard blocking: %v", err)
	}
}
