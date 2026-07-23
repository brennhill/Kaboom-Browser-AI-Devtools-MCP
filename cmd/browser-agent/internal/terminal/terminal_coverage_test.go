// terminal_coverage_test.go — Rejection and lifecycle paths for the terminal HTTP surface.
//
// The terminal server is unauthenticated apart from its own session tokens, so
// the guards tested here (method checks, token checks, body limits, session
// lookup) are the whole of its access control. The JSON keys asserted below are
// a wire contract with the side panel; renaming one breaks the UI silently.

package terminal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// covServerDeps is a ServerDeps whose stored path is observable.
type covServerDeps struct {
	mu   sync.Mutex
	path string
}

func (s *covServerDeps) GetActiveCodebase() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.path
}

func (s *covServerDeps) SetActiveCodebase(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = p
}

// covRelayMap records what the inject handler tried to write.
type covRelayMap struct {
	mu       sync.Mutex
	writes   [][]byte
	accept   bool
	closeAll int
}

func (m *covRelayMap) WriteToFirst(data []byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, append([]byte(nil), data...))
	return m.accept
}

func (m *covRelayMap) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeAll++
}

func (m *covRelayMap) written() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.writes))
	for i, w := range m.writes {
		out[i] = string(w)
	}
	return out
}

// covIntentDeps supplies the intent handlers with a relay map and a store.
// Both are stored as interfaces/pointers so a test can hand over a true nil —
// a typed nil would defeat the handler's own nil check.
type covIntentDeps struct {
	relays RelayMap
	store  *IntentStore
}

func (d covIntentDeps) GetPtyRelays() RelayMap       { return d.relays }
func (d covIntentDeps) GetIntentStore() *IntentStore { return d.store }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// covPost builds a POST request whose body is the JSON encoding of v, or the
// raw string when v is a string (so malformed bodies can be expressed).
func covPost(t *testing.T, path string, v any) *http.Request {
	t.Helper()
	var body []byte
	switch typed := v.(type) {
	case nil:
		body = nil
	case string:
		body = []byte(typed)
	default:
		enc, err := json.Marshal(typed)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		body = enc
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// covDecode decodes a recorded JSON body into a generic map.
func covDecode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	return out
}

// covAssertError checks status and the exact `error` code string, which the
// side panel switches on.
func covAssertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, status, rec.Body.String())
	}
	body := covDecode(t, rec)
	if body["error"] != code {
		t.Fatalf("error = %v, want %q (body=%s)", body["error"], code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /terminal/inject
// ---------------------------------------------------------------------------

func TestTerminalInject_RejectsNonPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, httptest.NewRequest("GET", "/terminal/inject", nil), testDeps(), covIntentDeps{})

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestTerminalInject_RejectsMalformedBody(t *testing.T) {
	relays := &covRelayMap{accept: true}
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, covPost(t, "/terminal/inject", "{not json"), testDeps(), covIntentDeps{relays: relays})

	covAssertError(t, rec, http.StatusBadRequest, "missing text field")
	// Nothing may reach the PTY when the body could not be parsed.
	if got := relays.written(); len(got) != 0 {
		t.Fatalf("wrote %q to the PTY despite a malformed body", got)
	}
}

func TestTerminalInject_RejectsEmptyText(t *testing.T) {
	relays := &covRelayMap{accept: true}
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, covPost(t, "/terminal/inject", map[string]string{"text": ""}), testDeps(), covIntentDeps{relays: relays})

	covAssertError(t, rec, http.StatusBadRequest, "missing text field")
	// An empty text would otherwise inject a bare newline and submit whatever
	// the user had half-typed at the prompt.
	if got := relays.written(); len(got) != 0 {
		t.Fatalf("wrote %q to the PTY for an empty text field", got)
	}
}

func TestTerminalInject_ReportsNoTerminalServer(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, covPost(t, "/terminal/inject", map[string]string{"text": "hi"}), testDeps(), covIntentDeps{relays: nil})

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := covDecode(t, rec)
	if body["injected"] != false {
		t.Fatalf("injected = %v, want false", body["injected"])
	}
	if body["reason"] != "no_terminal_server" {
		t.Fatalf("reason = %v, want %q", body["reason"], "no_terminal_server")
	}
}

func TestTerminalInject_ReportsNoActiveSession(t *testing.T) {
	relays := &covRelayMap{accept: false}
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, covPost(t, "/terminal/inject", map[string]string{"text": "hi"}), testDeps(), covIntentDeps{relays: relays})

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := covDecode(t, rec)
	// The extension distinguishes these two reasons: one means "start a
	// terminal", the other means "the daemon has no terminal server at all".
	if body["reason"] != "no_active_session" {
		t.Fatalf("reason = %v, want %q", body["reason"], "no_active_session")
	}
	if body["injected"] != false {
		t.Fatalf("injected = %v, want false", body["injected"])
	}
}

func TestTerminalInject_AppendsNewlineToInjectedText(t *testing.T) {
	relays := &covRelayMap{accept: true}
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, covPost(t, "/terminal/inject", map[string]string{"text": "find problems"}), testDeps(), covIntentDeps{relays: relays})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if body := covDecode(t, rec); body["injected"] != true {
		t.Fatalf("injected = %v, want true", body["injected"])
	}
	// Without the trailing newline the text sits unsubmitted at the prompt and
	// the agent never sees it.
	want := []string{"find problems\n"}
	got := relays.written()
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("injected %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// POST /intent
// ---------------------------------------------------------------------------

func TestIntentCreate_RejectsNonPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, httptest.NewRequest("GET", "/intent", nil), testDeps(), covIntentDeps{store: NewIntentStore()})

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestIntentCreate_RejectsMalformedBody(t *testing.T) {
	store := NewIntentStore()
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, covPost(t, "/intent", "{"), testDeps(), covIntentDeps{store: store})

	covAssertError(t, rec, http.StatusBadRequest, "invalid json")
	if n := len(store.Pending()); n != 0 {
		t.Fatalf("stored %d intents for a malformed body, want 0", n)
	}
}

func TestIntentCreate_ReportsUninitializedStore(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, covPost(t, "/intent", map[string]string{"page_url": "https://x.test/"}), testDeps(), covIntentDeps{store: nil})

	covAssertError(t, rec, http.StatusServiceUnavailable, "intent store not initialized")
}

func TestIntentCreate_DefaultsActionToQAScan(t *testing.T) {
	store := NewIntentStore()
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, covPost(t, "/intent", map[string]string{"page_url": "https://x.test/page"}), testDeps(), covIntentDeps{store: store})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	body := covDecode(t, rec)
	if body["stored"] != true {
		t.Fatalf("stored = %v, want true", body["stored"])
	}
	id, _ := body["correlation_id"].(string)
	if !strings.HasPrefix(id, "intent_") {
		t.Fatalf("correlation_id = %q, want an intent_ prefix", id)
	}

	pending := store.Pending()
	if len(pending) != 1 {
		t.Fatalf("pending = %d intents, want 1", len(pending))
	}
	// An intent with no action would never match the AI's qa_scan dispatch.
	if pending[0].Action != IntentActionQAScan {
		t.Fatalf("action = %q, want %q", pending[0].Action, IntentActionQAScan)
	}
	if pending[0].PageURL != "https://x.test/page" {
		t.Fatalf("page_url = %q, want the submitted URL", pending[0].PageURL)
	}
	if pending[0].CorrelationID != id {
		t.Fatalf("stored correlation_id %q != returned %q", pending[0].CorrelationID, id)
	}
}

func TestIntentCreate_PreservesExplicitAction(t *testing.T) {
	store := NewIntentStore()
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, covPost(t, "/intent", map[string]string{"action": "custom_scan"}), testDeps(), covIntentDeps{store: store})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	pending := store.Pending()
	if len(pending) != 1 || pending[0].Action != "custom_scan" {
		t.Fatalf("pending = %+v, want a single custom_scan intent", pending)
	}
}

// ---------------------------------------------------------------------------
// /config/active-codebase
// ---------------------------------------------------------------------------

func TestActiveCodebase_GetReturnsStoredPath(t *testing.T) {
	server := &covServerDeps{path: "/repos/kaboom"}
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, httptest.NewRequest("GET", "/config/active-codebase", nil), testDeps(), server)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := covDecode(t, rec)
	if body["active_codebase"] != "/repos/kaboom" {
		t.Fatalf("active_codebase = %v, want /repos/kaboom", body["active_codebase"])
	}
}

func TestActiveCodebase_PutTrimsSurroundingWhitespace(t *testing.T) {
	server := &covServerDeps{}
	req := covPost(t, "/config/active-codebase", map[string]string{"path": "  /repos/kaboom \n"})
	req.Method = "PUT"
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, req, testDeps(), server)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// A trailing newline survives a copy-paste from a terminal and would be
	// handed to fork/exec as part of the directory name.
	if got := server.GetActiveCodebase(); got != "/repos/kaboom" {
		t.Fatalf("stored path = %q, want it trimmed", got)
	}
	body := covDecode(t, rec)
	if body["status"] != "ok" || body["active_codebase"] != "/repos/kaboom" {
		t.Fatalf("body = %v, want status ok and the trimmed path echoed", body)
	}
}

func TestActiveCodebase_PostIsAcceptedLikePut(t *testing.T) {
	server := &covServerDeps{}
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, covPost(t, "/config/active-codebase", map[string]string{"path": "/tmp/x"}), testDeps(), server)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := server.GetActiveCodebase(); got != "/tmp/x" {
		t.Fatalf("stored path = %q, want /tmp/x", got)
	}
}

func TestActiveCodebase_RejectsMalformedBody(t *testing.T) {
	server := &covServerDeps{path: "/keep/me"}
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, covPost(t, "/config/active-codebase", "{\"path\":"), testDeps(), server)

	covAssertError(t, rec, http.StatusBadRequest, "invalid json")
	// A rejected write must not clear the previously configured codebase.
	if got := server.GetActiveCodebase(); got != "/keep/me" {
		t.Fatalf("stored path = %q, want it untouched", got)
	}
}

func TestActiveCodebase_RejectsUnsupportedMethod(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, httptest.NewRequest("DELETE", "/config/active-codebase", nil), testDeps(), &covServerDeps{})

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

// ---------------------------------------------------------------------------
// AutoDetectCWD
// ---------------------------------------------------------------------------

// covRegistry is a capture.ClientRegistry whose List() payload the test picks.
type covRegistry struct{ list any }

func (r covRegistry) Count() int                { return 0 }
func (r covRegistry) List() any                 { return r.list }
func (r covRegistry) Register(cwd string) any   { return nil }
func (r covRegistry) Get(id string) any         { return nil }
func (r covRegistry) Unregister(id string) bool { return false }

func covStoreWithRegistry(list any) *capture.Store {
	store := capture.NewCapture()
	store.SetClientRegistry(covRegistry{list: list})
	return store
}

func TestAutoDetectCWD_EmptyWithoutRegistry(t *testing.T) {
	if got := AutoDetectCWD(capture.NewCapture()); got != "" {
		t.Fatalf("AutoDetectCWD = %q, want empty when no client registry is wired", got)
	}
}

func TestAutoDetectCWD_EmptyWhenRegistryListsNothing(t *testing.T) {
	if got := AutoDetectCWD(covStoreWithRegistry(nil)); got != "" {
		t.Fatalf("AutoDetectCWD = %q, want empty when List() returns nil", got)
	}
}

func TestAutoDetectCWD_SkipsClientsWithoutCWD(t *testing.T) {
	// A client that registered before its cwd was known must not win and leave
	// the terminal spawning in the daemon's own working directory.
	store := covStoreWithRegistry([]any{
		map[string]any{"id": "a"},
		map[string]any{"id": "b", "cwd": ""},
		map[string]any{"id": "c", "cwd": "/repos/kaboom"},
	})
	if got := AutoDetectCWD(store); got != "/repos/kaboom" {
		t.Fatalf("AutoDetectCWD = %q, want /repos/kaboom", got)
	}
}

func TestAutoDetectCWD_FallsBackToJSONRoundtrip(t *testing.T) {
	// The registry is declared as `any`, so a concrete slice type is the normal
	// case; the roundtrip is what makes the snake_case `cwd` tag readable here.
	type client struct {
		ID  string `json:"id"`
		CWD string `json:"cwd"`
	}
	store := covStoreWithRegistry([]client{{ID: "a"}, {ID: "b", CWD: "/srv/app"}})
	if got := AutoDetectCWD(store); got != "/srv/app" {
		t.Fatalf("AutoDetectCWD = %q, want /srv/app from the JSON fallback", got)
	}
}

func TestAutoDetectCWD_EmptyWhenPayloadIsNotAClientList(t *testing.T) {
	// Marshals fine, does not unmarshal into a list of objects.
	if got := AutoDetectCWD(covStoreWithRegistry("not-a-list")); got != "" {
		t.Fatalf("AutoDetectCWD = %q, want empty for an unusable payload", got)
	}
}

func TestAutoDetectCWD_EmptyWhenPayloadIsUnmarshalable(t *testing.T) {
	if got := AutoDetectCWD(covStoreWithRegistry(make(chan int))); got != "" {
		t.Fatalf("AutoDetectCWD = %q, want empty when the payload cannot be marshalled", got)
	}
}

// ---------------------------------------------------------------------------
// HandleControlMessage
//
// A nil *pty.Session is deliberate: every case below must return before
// touching the session, so a regression that drops a guard panics here.
// ---------------------------------------------------------------------------

func TestControlMessage_IgnoresMalformedJSON(t *testing.T) {
	HandleControlMessage([]byte("{not json"), nil)
}

func TestControlMessage_IgnoresUnknownType(t *testing.T) {
	HandleControlMessage([]byte(`{"type":"paste","cols":80,"rows":24}`), nil)
}

func TestControlMessage_IgnoresNonPositiveDimensions(t *testing.T) {
	// A browser tab that is hidden reports 0x0; resizing a PTY to 0 columns
	// wedges every TUI running inside it.
	for _, payload := range []string{
		`{"type":"resize","cols":0,"rows":24}`,
		`{"type":"resize","cols":80,"rows":0}`,
		`{"type":"resize","cols":-1,"rows":-1}`,
	} {
		HandleControlMessage([]byte(payload), nil)
	}
}

// ---------------------------------------------------------------------------
// POST /terminal/start
// ---------------------------------------------------------------------------

// covShell is a command that stays alive without producing output, so a session
// exists for the duration of a test and dies with the manager.
var covShell = pty.StartConfig{Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}}

func covStartCfg(id, dir string) pty.StartConfig {
	cfg := covShell
	cfg.ID = id
	cfg.Dir = dir
	return cfg
}

func TestTerminalStart_RejectsNonPOST(t *testing.T) {
	mgr := pty.NewManager()
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, httptest.NewRequest("GET", "/terminal/start", nil), testDeps(), nil, mgr, nil, NewMap())

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if mgr.Count() != 0 {
		t.Fatal("a rejected method must not spawn a PTY")
	}
}

func TestTerminalStart_RejectsMalformedBody(t *testing.T) {
	mgr := pty.NewManager()
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, covPost(t, "/terminal/start", "{\"cmd\":"), testDeps(), nil, mgr, nil, NewMap())

	covAssertError(t, rec, http.StatusBadRequest, "invalid json")
	if mgr.Count() != 0 {
		t.Fatal("a malformed body must not spawn a PTY")
	}
}

func TestTerminalStart_RejectsBodyOverTheLimit(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()
	deps.MaxPostBody = 32 // smaller than the body below

	oversized := map[string]any{"cmd": "/bin/sh", "dir": strings.Repeat("x", 512)}
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, covPost(t, "/terminal/start", oversized), deps, nil, mgr, nil, NewMap())

	// MaxBytesReader truncates the stream, so the decoder is what reports it —
	// the point is that an oversized body is refused rather than buffered.
	covAssertError(t, rec, http.StatusBadRequest, "invalid json")
	if mgr.Count() != 0 {
		t.Fatal("an oversized body must not spawn a PTY")
	}
}

func TestTerminalStart_UsesActiveCodebaseWhenDirOmitted(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	dir := t.TempDir()
	relays := NewMap()

	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, covPost(t, "/terminal/start", map[string]any{
		"cmd": "/bin/sh", "args": []string{"-c", "exec cat"},
	}), testDeps(), &covServerDeps{path: dir}, mgr, nil, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	relay := relays.Get("default")
	if relay == nil {
		t.Fatal("a successful start must register a relay for the session")
	}
	// The workspace dir is what /terminal/upload writes into; losing the active
	// codebase here silently drops uploads into the wrong tree.
	if got := relay.WorkspaceDir(); got != dir {
		t.Fatalf("workspace dir = %q, want the active codebase %q", got, dir)
	}
}

func TestTerminalStart_RequestDirWinsOverActiveCodebase(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	requested := t.TempDir()
	relays := NewMap()

	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, covPost(t, "/terminal/start", map[string]any{
		"cmd": "/bin/sh", "args": []string{"-c", "exec cat"}, "dir": requested,
	}), testDeps(), &covServerDeps{path: t.TempDir()}, mgr, nil, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := relays.Get("default").WorkspaceDir(); got != requested {
		t.Fatalf("workspace dir = %q, want the explicitly requested %q", got, requested)
	}
}

func TestTerminalStart_SessionLimitReturnsConflictWithoutToken(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	relays := NewMap()
	defer relays.CloseAll()

	// Fill the manager to its concurrent-session limit.
	for i := 0; mgr.Count() == i; i++ {
		if _, err := mgr.Start(covStartCfg("filler-"+string(rune('a'+i)), "")); err != nil {
			break
		}
	}
	if mgr.Count() == 0 {
		t.Fatal("could not start any session")
	}

	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, covPost(t, "/terminal/start", map[string]any{
		"id": "over-limit", "cmd": "/bin/sh", "args": []string{"-c", "exec cat"},
	}), testDeps(), nil, mgr, nil, relays)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	body := covDecode(t, rec)
	// The client reconnects using session_id/token, so both keys must be present
	// even on failure — a bare error body leaves the panel with nothing to do.
	if body["session_id"] != "over-limit" {
		t.Fatalf("session_id = %v, want the requested id", body["session_id"])
	}
	if body["token"] != "" {
		t.Fatalf("token = %v, want empty when no such session exists", body["token"])
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "maximum concurrent sessions") {
		t.Fatalf("error = %q, want the real limit error preserved", msg)
	}
}

// ---------------------------------------------------------------------------
// POST /terminal/stop
// ---------------------------------------------------------------------------

func TestTerminalStop_RejectsNonPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalStop(rec, httptest.NewRequest("GET", "/terminal/stop", nil), testDeps(), pty.NewManager(), NewMap())

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestTerminalStop_RejectsMalformedBody(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(covStartCfg("default", "")); err != nil {
		t.Fatalf("start: %v", err)
	}

	rec := httptest.NewRecorder()
	HandleTerminalStop(rec, covPost(t, "/terminal/stop", "{"), testDeps(), mgr, NewMap())

	covAssertError(t, rec, http.StatusBadRequest, "invalid json")
	// A parse failure must never be treated as "stop everything".
	if mgr.Count() != 1 {
		t.Fatalf("sessions = %d, want the session left running", mgr.Count())
	}
}

func TestTerminalStop_DefaultsToTheDefaultSessionAndDropsItsRelay(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(covStartCfg("default", "")); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("default")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	relays.GetOrCreate("default", sess, t.TempDir())

	rec := httptest.NewRecorder()
	HandleTerminalStop(rec, covPost(t, "/terminal/stop", map[string]string{}), testDeps(), mgr, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if body := covDecode(t, rec); body["status"] != "stopped" {
		t.Fatalf("status field = %v, want \"stopped\"", body["status"])
	}
	if mgr.Count() != 0 {
		t.Fatalf("sessions = %d, want 0 — an omitted id means the default session", mgr.Count())
	}
	// Leaving the relay behind leaks its reader goroutine and makes a later
	// start reuse a relay bound to a dead PTY.
	if relays.Get("default") != nil {
		t.Fatal("stopping a session must remove its relay")
	}
}

// ---------------------------------------------------------------------------
// GET /terminal/config and GET /terminal/validate
// ---------------------------------------------------------------------------

func TestTerminalConfig_RejectsNonGET(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalConfig(rec, httptest.NewRequest("DELETE", "/terminal/config", nil), testDeps(), pty.NewManager(), NewMap())

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestTerminalConfig_EmptyListIsAnArrayNotNull(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalConfig(rec, httptest.NewRequest("GET", "/terminal/config", nil), testDeps(), pty.NewManager(), NewMap())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// `null` would make the panel's sessions.map() throw instead of rendering
	// an empty list, so the encoded shape matters, not just the decoded value.
	if !strings.Contains(rec.Body.String(), `"sessions":[]`) {
		t.Fatalf("body = %s, want an empty sessions array", rec.Body.String())
	}
	if body := covDecode(t, rec); body["count"] != float64(0) {
		t.Fatalf("count = %v, want 0", body["count"])
	}
}

func TestTerminalConfig_ReportsSubscriberCountOnlyForRelayedSessions(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	for _, id := range []string{"with-relay", "no-relay"} {
		if _, err := mgr.Start(covStartCfg(id, "")); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	sess, err := mgr.Get("with-relay")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	relay := relays.GetOrCreate("with-relay", sess, "")
	defer relays.CloseAll()
	for _, id := range []string{"viewer-1", "viewer-2"} {
		if _, err := relay.Fanout().Subscribe(id); err != nil {
			t.Fatalf("subscribe %s: %v", id, err)
		}
	}

	rec := httptest.NewRecorder()
	HandleTerminalConfig(rec, httptest.NewRequest("GET", "/terminal/config", nil), testDeps(), mgr, relays)

	body := covDecode(t, rec)
	sessions, ok := body["sessions"].([]any)
	if !ok || len(sessions) != 2 {
		t.Fatalf("sessions = %v, want 2 entries", body["sessions"])
	}
	seen := map[string]any{}
	for _, s := range sessions {
		info := s.(map[string]any)
		seen[info["id"].(string)] = info["subscribers"]
	}
	// The panel uses this to warn about a terminal open in another tab.
	if seen["with-relay"] != float64(2) {
		t.Fatalf("subscribers = %v, want 2", seen["with-relay"])
	}
	if seen["no-relay"] != nil {
		t.Fatalf("subscribers = %v for a session with no relay, want the key omitted", seen["no-relay"])
	}
}

func TestTerminalValidate_RejectsNonGET(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalValidate(rec, httptest.NewRequest("POST", "/terminal/validate?token=x", nil), testDeps(), pty.NewManager())

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

// ---------------------------------------------------------------------------
// /terminal/ws and /terminal/upload rejection paths
// ---------------------------------------------------------------------------

func TestTerminalWS_RequiresAHijackableResponseWriter(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	result, err := mgr.Start(covShell)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/ws?token="+result.Token, nil)
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Upgrade", "WebSocket") // case-insensitive per RFC 6455
	rec := httptest.NewRecorder()
	HandleTerminalWS(rec, req, testDeps(), mgr, NewMap())

	// httptest.ResponseRecorder is not an http.Hijacker, which is exactly the
	// condition the handler must report rather than panic on.
	covAssertError(t, rec, http.StatusInternalServerError, "server does not support hijacking")
}

func TestTerminalWS_RejectsUpgradeWithoutWebSocketKey(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	result, err := mgr.Start(covShell)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/ws?token="+result.Token, nil)
	req.Header.Set("Upgrade", "websocket") // key deliberately missing
	rec := httptest.NewRecorder()
	HandleTerminalWS(rec, req, testDeps(), mgr, NewMap())

	covAssertError(t, rec, http.StatusBadRequest, "websocket upgrade required")
}

func TestTerminalUpload_RejectsNonPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleTerminalUpload(rec, httptest.NewRequest("GET", "/terminal/upload", nil), testDeps(), pty.NewManager(), NewMap())

	covAssertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestTerminalUpload_MissingSessionIDFallsBackToDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/terminal/upload", bytes.NewReader([]byte("data")))
	req.Header.Set("Content-Type", "image/png")
	HandleTerminalUpload(rec, req, testDeps(), pty.NewManager(), NewMap())

	// No session_id means the default session; with none running that is a 404,
	// not a 200 that writes into an unrelated session's workspace.
	covAssertError(t, rec, http.StatusNotFound, "session not found")
}

func TestTerminalUpload_RequiresAWorkspaceDirectory(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(covStartCfg("no-workspace", "")); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("no-workspace")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	defer relays.CloseAll()
	relays.GetOrCreate("no-workspace", sess, "") // started without a dir

	req := httptest.NewRequest("POST", "/terminal/upload?session_id=no-workspace&filename=a.png", bytes.NewReader([]byte("data")))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	HandleTerminalUpload(rec, req, testDeps(), mgr, relays)

	// Without a workspace the daemon has nowhere safe to put the file; falling
	// back to the process cwd would scatter uploads into the install directory.
	covAssertError(t, rec, http.StatusBadRequest, "no workspace directory for session")
}

// ---------------------------------------------------------------------------
// Relay map
// ---------------------------------------------------------------------------

func TestRelayMap_WriteToFirstReportsNoRelay(t *testing.T) {
	// The inject handler turns this false into the "no_active_session" reason.
	if NewMap().WriteToFirst([]byte("hello\n")) {
		t.Fatal("WriteToFirst must report false when no session is relayed")
	}
}

func TestRelayMap_WriteToFirstReachesThePTY(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(covStartCfg("writer", "")); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("writer")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	defer relays.CloseAll()
	relays.GetOrCreate("writer", sess, "")

	if !relays.WriteToFirst([]byte("kaboom-marker\n")) {
		t.Fatal("WriteToFirst must report true once a relay exists")
	}
	// `cat` echoes stdin back through the PTY, so the marker reappearing in
	// scrollback proves the bytes travelled write buffer -> PTY -> read loop.
	testsync.Eventually(t, testsync.DefaultTimeout, "the injected text to echo back through the PTY", func() bool {
		return bytes.Contains(sess.Scrollback(), []byte("kaboom-marker"))
	})
}

func TestRelayMap_CloseAllForgetsEveryRelay(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	relays := NewMap()
	for _, id := range []string{"a", "b"} {
		if _, err := mgr.Start(covStartCfg(id, "")); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
		sess, err := mgr.Get(id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		relays.GetOrCreate(id, sess, "")
	}

	relays.CloseAll()

	for _, id := range []string{"a", "b"} {
		if relays.Get(id) != nil {
			t.Fatalf("relay %q survived CloseAll", id)
		}
	}
	// A stale relay would still be handed out on the next WriteToFirst.
	if relays.WriteToFirst([]byte("x")) {
		t.Fatal("WriteToFirst must find nothing after CloseAll")
	}
}

func TestRelayMap_GetOrCreateIsIdempotentPerSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(covStartCfg("shared", "")); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("shared")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	defer relays.CloseAll()

	first := relays.GetOrCreate("shared", sess, "/one")
	second := relays.GetOrCreate("shared", sess, "/two")

	// A second relay would start a second read loop and split the PTY output
	// between two fan-outs, so each viewer would see half the terminal.
	if first != second {
		t.Fatal("GetOrCreate must return the existing relay for a session")
	}
	if got := second.WorkspaceDir(); got != "/one" {
		t.Fatalf("workspace dir = %q, want the original %q", got, "/one")
	}
}

func TestNextWSSubID_IsUniqueUnderConcurrency(t *testing.T) {
	const n = 64
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids[i] = NextWSSubID()
		}(i)
	}
	wg.Wait()

	// Two connections sharing an id would unsubscribe each other from the
	// fan-out, blanking one browser tab whenever another disconnects.
	seen := make(map[string]bool, n)
	for _, id := range ids {
		if !strings.HasPrefix(id, "ws-") {
			t.Fatalf("subscriber id %q lost its ws- prefix", id)
		}
		if seen[id] {
			t.Fatalf("duplicate subscriber id %q", id)
		}
		seen[id] = true
	}
}

// ---------------------------------------------------------------------------
// Relay lifecycle
// ---------------------------------------------------------------------------

func TestRelay_CapturesTheChildExitCode(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(pty.StartConfig{ID: "exiting", Cmd: "/bin/sh", Args: []string{"-c", "exit 7"}}); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("exiting")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relay := NewRelay(sess, "")

	testsync.Eventually(t, testsync.DefaultTimeout, "the relay read loop to finish after the child exits", func() bool {
		select {
		case <-relay.done:
			return true
		default:
			return false
		}
	})
	// The browser prints this code in the "session exited" notice; losing it
	// makes every crash look like a clean exit.
	if got := relay.ExitCode(); got != 7 {
		t.Fatalf("exit code = %d, want 7", got)
	}
}

func TestWaitForPromptViaRelay_WritesInitCommandWhenFanoutIsClosed(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	if _, err := mgr.Start(covStartCfg("init-cmd", "")); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("init-cmd")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relay := NewRelay(sess, "")

	// A fan-out that is already closed is what a reconnect after process exit
	// looks like; the init command must still be delivered rather than dropped.
	relay.Fanout().Close()
	WaitForPromptViaRelay(relay, "kaboom-init")

	testsync.Eventually(t, testsync.DefaultTimeout, "the init command to reach the PTY", func() bool {
		return bytes.Contains(sess.Scrollback(), []byte("kaboom-init"))
	})
}

// ---------------------------------------------------------------------------
// Route wiring
// ---------------------------------------------------------------------------

func TestRegisterRoutes_WiresTheTerminalEndpoints(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	mux := http.NewServeMux()
	relays := RegisterRoutes(mux, testDeps(), nil, mgr, nil)
	defer relays.CloseAll()

	cases := []struct {
		method, path string
		status       int
	}{
		{"GET", "/terminal", http.StatusOK},
		{"POST", "/terminal", http.StatusMethodNotAllowed},
		{"POST", "/terminal/dirs", http.StatusMethodNotAllowed},
		{"GET", "/terminal/validate?token=bogus", http.StatusOK},
		{"GET", "/terminal/ws", http.StatusUnauthorized},
		{"GET", "/terminal/start", http.StatusMethodNotAllowed},
		{"GET", "/terminal/stop", http.StatusMethodNotAllowed},
		{"GET", "/terminal/upload", http.StatusMethodNotAllowed},
		{"DELETE", "/terminal/config", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		// An unregistered route answers 404, so these codes prove the wiring.
		if rec.Code != tc.status {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
		}
	}
}

func TestRegisterRoutes_ServesStaticAssetsWithThePrefixStripped(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	mux := http.NewServeMux()
	relays := RegisterRoutes(mux, testDeps(), nil, mgr, nil)
	defer relays.CloseAll()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/terminal/static/xterm.min.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("xterm.min.js = %d, want 200 — the terminal page cannot boot without it", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("xterm.min.js served an empty body")
	}

	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest("GET", "/terminal/static/not-a-real-asset.js", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing asset = %d, want 404", missing.Code)
	}
}

func TestSetupMux_WiresIntentRoutesAlongsideTerminalRoutes(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	store := NewIntentStore()
	mux, relays := SetupMux(testDeps(), nil, covIntentDeps{store: store, relays: &covRelayMap{accept: true}}, mgr, nil)
	defer relays.CloseAll()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, covPost(t, "/intent", map[string]string{"page_url": "https://x.test/"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /intent = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if len(store.Pending()) != 1 {
		t.Fatal("POST /intent through the mux did not reach the store")
	}

	inject := httptest.NewRecorder()
	mux.ServeHTTP(inject, covPost(t, "/terminal/inject", map[string]string{"text": "hi"}))
	if inject.Code != http.StatusOK {
		t.Fatalf("POST /terminal/inject = %d, want 200 (body=%s)", inject.Code, inject.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Terminal HTTP server
// ---------------------------------------------------------------------------

func TestStartServer_ReportsABindFailure(t *testing.T) {
	// Occupy a loopback port so the terminal server cannot have it. This is the
	// daemon-already-running case, which must surface as an error rather than a
	// server that silently never listens.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind a loopback port here: %v", err)
	}
	defer func() { _ = blocker.Close() }()
	port := blocker.Addr().(*net.TCPAddr).Port

	srv, done, err := StartServer(testDeps(), port, http.NewServeMux())
	if err == nil {
		_ = srv.Close()
		t.Fatal("StartServer must fail when the port is taken")
	}
	if srv != nil || done != nil {
		t.Fatal("a failed StartServer must not hand back a server or a done channel")
	}
	if !strings.Contains(err.Error(), "cannot bind port") {
		t.Fatalf("error = %q, want it to name the bind failure", err)
	}
}

func TestStartServer_ClosesDoneOnShutdown(t *testing.T) {
	srv, done, err := StartServer(testDeps(), 0, http.NewServeMux()) // port 0 = any free loopback port
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	select {
	case <-done:
		t.Fatal("done closed while the server was still listening")
	default:
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// The daemon watches this channel to know the terminal port died; if it
	// never closes, a dead listener looks healthy forever.
	testsync.Eventually(t, testsync.DefaultTimeout, "the terminal server done channel to close", func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
}

// ---------------------------------------------------------------------------
// WebSocket relay (handshake + wsLoop)
//
// These use a loopback httptest server because http.ResponseWriter must be an
// http.Hijacker for the upgrade to happen at all — a recorder cannot reach the
// relay loop. No DNS and no traffic leaves the machine.
// ---------------------------------------------------------------------------

// covWSKey is the RFC 6455 example key; covWSAccept is its required answer.
const (
	covWSKey    = "dGhlIHNhbXBsZSBub25jZQ=="
	covWSAccept = "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
)

type covWSFixture struct {
	mgr    *pty.Manager
	relays *Map
	sess   *pty.Session
	relay  *Relay
	srv    *httptest.Server
	token  string
}

// covWSServe starts a session, its relay, and an HTTP server bound to
// HandleTerminalWS. The relay is created up front so the read loop is already
// draining the PTY into scrollback before the browser connects.
func covWSServe(t *testing.T, shellScript string) *covWSFixture {
	t.Helper()
	mgr := pty.NewManager()
	result, err := mgr.Start(pty.StartConfig{ID: "ws", Cmd: "/bin/sh", Args: []string{"-c", shellScript}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("ws")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	relay := relays.GetOrCreate("ws", sess, "")
	deps := testDeps()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalWS(w, r, deps, mgr, relays)
	}))
	t.Cleanup(func() {
		srv.Close()
		relays.CloseAll()
		mgr.StopAll()
	})
	return &covWSFixture{mgr: mgr, relays: relays, sess: sess, relay: relay, srv: srv, token: result.Token}
}

// covWSDial performs the upgrade handshake and returns the raw connection.
func covWSDial(t *testing.T, f *covWSFixture, token string) (net.Conn, *bufio.Reader) {
	t.Helper()
	addr := f.srv.Listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(testsync.DefaultTimeout))

	handshake := "GET /terminal/ws?token=" + token + " HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + covWSKey + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(handshake)); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake status = %d, want 101", resp.StatusCode)
	}
	// A browser aborts the connection if this digest is wrong, so it is a hard
	// wire contract rather than an implementation detail.
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != covWSAccept {
		t.Fatalf("Sec-WebSocket-Accept = %q, want %q", got, covWSAccept)
	}
	return conn, br
}

// covWSSend writes one client frame, masked the way a browser masks.
func covWSSend(t *testing.T, conn net.Conn, fin bool, opcode byte, payload []byte) {
	t.Helper()
	first := opcode
	if fin {
		first |= 0x80
	}
	frame := []byte{first}
	n := len(payload)
	switch {
	case n < 126:
		frame = append(frame, 0x80|byte(n))
	default:
		frame = append(frame, 0x80|126, byte(n>>8), byte(n))
	}
	mask := []byte{0x37, 0xfa, 0x21, 0x3d}
	frame = append(frame, mask...)
	for i := 0; i < n; i++ {
		frame = append(frame, payload[i]^mask[i%4])
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

// covWSReadUntil reads server frames until one carries the wanted opcode.
func covWSReadUntil(t *testing.T, br *bufio.Reader, want byte, what string) []byte {
	t.Helper()
	for {
		fin, opcode, payload, err := testWSReadFrame(br)
		if err != nil {
			t.Fatalf("waiting for %s: %v", what, err)
		}
		if !fin {
			t.Fatalf("server sent a fragmented frame while waiting for %s", what)
		}
		if opcode == want {
			return payload
		}
	}
}

func TestTerminalWS_ReplaysScrollbackThenSignalsReplayEnd(t *testing.T) {
	f := covWSServe(t, "printf kaboom-ready; exec cat")
	testsync.Eventually(t, testsync.DefaultTimeout, "the shell banner to reach scrollback", func() bool {
		return bytes.Contains(f.sess.Scrollback(), []byte("kaboom-ready"))
	})

	_, br := covWSDial(t, f, f.token)

	var replayed []byte
	for {
		_, opcode, payload, err := testWSReadFrame(br)
		if err != nil {
			t.Fatalf("read replay: %v", err)
		}
		if opcode == 0x2 {
			replayed = append(replayed, payload...)
			continue
		}
		// The first text frame ends the replay; the browser clears its
		// "reconnecting" state on it, so its exact shape is a contract.
		if string(payload) != `{"type":"replay_end"}` {
			t.Fatalf("first text frame = %s, want the replay_end marker", payload)
		}
		break
	}
	// Without replay, a page refresh presents an empty terminal even though the
	// PTY behind it still holds the whole session.
	if !bytes.Contains(replayed, []byte("kaboom-ready")) {
		t.Fatalf("replayed %q, want the prior output", replayed)
	}
}

func TestTerminalWS_RelaysKeystrokesToThePTY(t *testing.T) {
	f := covWSServe(t, "exec cat")
	conn, br := covWSDial(t, f, f.token)
	covWSReadUntil(t, br, 0x1, "the replay_end marker")

	covWSSend(t, conn, true, 0x2, []byte("kaboom-typed\n"))

	testsync.Eventually(t, testsync.DefaultTimeout, "the keystrokes to reach the PTY", func() bool {
		return bytes.Contains(f.sess.Scrollback(), []byte("kaboom-typed"))
	})
}

func TestTerminalWS_AnswersPingWithPong(t *testing.T) {
	f := covWSServe(t, "exec cat")
	conn, br := covWSDial(t, f, f.token)
	covWSReadUntil(t, br, 0x1, "the replay_end marker")

	covWSSend(t, conn, true, 0x9, []byte("pp"))

	// RFC 6455 requires the pong to echo the ping payload verbatim.
	if got := covWSReadUntil(t, br, 0xA, "a pong"); string(got) != "pp" {
		t.Fatalf("pong payload = %q, want the ping payload back", got)
	}
}

func TestTerminalWS_RejectsFragmentedFrames(t *testing.T) {
	f := covWSServe(t, "exec cat")
	conn, br := covWSDial(t, f, f.token)
	covWSReadUntil(t, br, 0x1, "the replay_end marker")

	covWSSend(t, conn, false, 0x2, []byte("half-a-com"))

	// Reassembly is not implemented, so accepting a fragment would write a
	// truncated command into the shell. The server must close instead.
	covWSReadUntil(t, br, 0x8, "a close frame")
	if bytes.Contains(f.sess.Scrollback(), []byte("half-a-com")) {
		t.Fatal("a fragmented frame must not be written to the PTY")
	}
}

func TestTerminalWS_ClosesWhenTheBrowserSendsClose(t *testing.T) {
	f := covWSServe(t, "exec cat")
	conn, br := covWSDial(t, f, f.token)
	covWSReadUntil(t, br, 0x1, "the replay_end marker")

	covWSSend(t, conn, true, 0x8, nil)
	covWSReadUntil(t, br, 0x8, "the close handshake reply")

	// The PTY must outlive the socket: a page refresh closes the WebSocket and
	// the session has to be there to reconnect to.
	if !f.sess.IsAlive() {
		t.Fatal("closing the WebSocket killed the PTY session")
	}
}

func TestTerminalWS_NotifiesTheBrowserWhenTheSessionAlreadyExited(t *testing.T) {
	f := covWSServe(t, "exit 3")
	testsync.Eventually(t, testsync.DefaultTimeout, "the relay to finish after the child exits", func() bool {
		select {
		case <-f.relay.done:
			return true
		default:
			return false
		}
	})

	_, br := covWSDial(t, f, f.token)
	covWSReadUntil(t, br, 0x1, "the replay_end marker")

	// Subscribing to a closed fan-out is the "process already gone" case. The
	// browser needs the exit notice or it reconnects in a loop forever.
	exited := covWSReadUntil(t, br, 0x1, "the exited notice")
	var msg struct {
		Type string `json:"type"`
		Code int    `json:"code"`
	}
	if err := json.Unmarshal(exited, &msg); err != nil {
		t.Fatalf("decode exit notice %q: %v", exited, err)
	}
	if msg.Type != "exited" || msg.Code != 3 {
		t.Fatalf("exit notice = %+v, want type exited with code 3", msg)
	}
	covWSReadUntil(t, br, 0x8, "the close frame after the exit notice")
}
