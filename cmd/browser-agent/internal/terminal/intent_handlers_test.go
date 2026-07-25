// intent_handlers_test.go -- Tests for intent creation and terminal injection
// handlers using in-memory fakes (no PTY, no real server).

package terminal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A valid-JSON body larger than MaxPostBody must be bounded by MaxBytesReader and
// rejected, not fully buffered — same cap every other terminal handler applies (G).
func TestHandleTerminalInject_CapsBodySize(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.MaxPostBody = 1024
	relays := &fakeRelayMap{writeOK: true}

	big := strings.Repeat("A", 8192)
	body := fmt.Sprintf(`{"text":%q}`, big)
	req := httptest.NewRequest("POST", "/terminal/inject", strings.NewReader(body))
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, req, deps, &fakeIntentDeps{relays: relays})

	if rec.Code == http.StatusOK {
		t.Fatalf("oversized inject body must be bounded/rejected, got 200")
	}
	if len(relays.written) != 0 {
		t.Fatalf("an oversized body must be rejected before injecting into the PTY, got %d writes", len(relays.written))
	}
}

func TestHandleIntentCreate_CapsBodySize(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.MaxPostBody = 1024
	store := NewIntentStore()

	big := strings.Repeat("A", 8192)
	body := fmt.Sprintf(`{"page_url":%q,"action":"qa_scan"}`, big)
	req := httptest.NewRequest("POST", "/intent", strings.NewReader(body))
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, req, deps, &fakeIntentDeps{store: store})

	if rec.Code == http.StatusOK {
		t.Fatalf("oversized intent body must be bounded/rejected, got 200")
	}
	if len(store.Pending()) != 0 {
		t.Fatalf("an oversized body must be rejected before storing an intent, got %d", len(store.Pending()))
	}
}

func TestHandleTerminalInject_Success(t *testing.T) {
	t.Parallel()
	relays := &fakeRelayMap{writeOK: true}
	deps := testDeps()
	intentDeps := &fakeIntentDeps{relays: relays}

	body, _ := json.Marshal(map[string]string{"text": "run tests"})
	req := httptest.NewRequest("POST", "/terminal/inject", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, req, deps, intentDeps)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["injected"] != true {
		t.Fatalf("expected injected=true, got %v", resp["injected"])
	}
	if len(relays.written) != 1 || string(relays.written[0]) != "run tests\n" {
		t.Fatalf("expected newline-terminated write, got %q", relays.written)
	}
}

func TestHandleTerminalInject_NoActiveSession(t *testing.T) {
	t.Parallel()
	relays := &fakeRelayMap{writeOK: false}
	deps := testDeps()
	intentDeps := &fakeIntentDeps{relays: relays}

	body, _ := json.Marshal(map[string]string{"text": "hi"})
	req := httptest.NewRequest("POST", "/terminal/inject", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, req, deps, intentDeps)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["reason"] != "no_active_session" {
		t.Fatalf("expected reason no_active_session, got %v", resp["reason"])
	}
}

func TestHandleTerminalInject_NoTerminalServer(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	// relays is a nil RelayMap interface value.
	intentDeps := &fakeIntentDeps{relays: nil}

	body, _ := json.Marshal(map[string]string{"text": "hi"})
	req := httptest.NewRequest("POST", "/terminal/inject", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, req, deps, intentDeps)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["reason"] != "no_terminal_server" {
		t.Fatalf("expected reason no_terminal_server, got %v", resp["reason"])
	}
}

func TestHandleTerminalInject_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	req := httptest.NewRequest("GET", "/terminal/inject", nil)
	rec := httptest.NewRecorder()
	HandleTerminalInject(rec, req, deps, &fakeIntentDeps{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleTerminalInject_BadBody(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	cases := []struct {
		name string
		body string
	}{
		{"invalid json", "{"},
		{"empty text", `{"text":""}`},
		{"missing text field", `{"other":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/terminal/inject", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			HandleTerminalInject(rec, req, deps, &fakeIntentDeps{relays: &fakeRelayMap{writeOK: true}})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d", tc.name, rec.Code)
			}
		})
	}
}

func TestHandleIntentCreate_Success(t *testing.T) {
	t.Parallel()
	store := NewIntentStore()
	deps := testDeps()
	intentDeps := &fakeIntentDeps{store: store}

	body, _ := json.Marshal(map[string]string{"page_url": "http://localhost:3000", "action": "custom_scan"})
	req := httptest.NewRequest("POST", "/intent", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, req, deps, intentDeps)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["stored"] != true {
		t.Fatalf("expected stored=true, got %v", resp["stored"])
	}
	cid, _ := resp["correlation_id"].(string)
	if !strings.HasPrefix(cid, "intent_") {
		t.Fatalf("expected intent_ correlation id, got %q", cid)
	}
	pending := store.Pending()
	if len(pending) != 1 || pending[0].Action != "custom_scan" {
		t.Fatalf("expected stored custom_scan intent, got %+v", pending)
	}
}

func TestHandleIntentCreate_DefaultsAction(t *testing.T) {
	t.Parallel()
	store := NewIntentStore()
	deps := testDeps()

	body, _ := json.Marshal(map[string]string{"page_url": "http://localhost:3000"})
	req := httptest.NewRequest("POST", "/intent", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, req, deps, &fakeIntentDeps{store: store})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	pending := store.Pending()
	if len(pending) != 1 || pending[0].Action != IntentActionQAScan {
		t.Fatalf("expected default action %q, got %+v", IntentActionQAScan, pending)
	}
}

func TestHandleIntentCreate_StoreNotInitialized(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	body, _ := json.Marshal(map[string]string{"page_url": "http://x"})
	req := httptest.NewRequest("POST", "/intent", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, req, deps, &fakeIntentDeps{store: nil})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleIntentCreate_MethodAndBadJSON(t *testing.T) {
	t.Parallel()
	deps := testDeps()

	req := httptest.NewRequest("GET", "/intent", nil)
	rec := httptest.NewRecorder()
	HandleIntentCreate(rec, req, deps, &fakeIntentDeps{store: NewIntentStore()})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	req = httptest.NewRequest("POST", "/intent", strings.NewReader("{bad"))
	rec = httptest.NewRecorder()
	HandleIntentCreate(rec, req, deps, &fakeIntentDeps{store: NewIntentStore()})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterIntentRoutes_Dispatch(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	store := NewIntentStore()
	intentDeps := &fakeIntentDeps{relays: &fakeRelayMap{writeOK: true}, store: store}

	mux := http.NewServeMux()
	RegisterIntentRoutes(mux, deps, intentDeps)

	// /intent route wired.
	body, _ := json.Marshal(map[string]string{"page_url": "http://localhost", "action": "qa_scan"})
	req := httptest.NewRequest("POST", "/intent", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/intent: expected 200, got %d", rec.Code)
	}

	// /terminal/inject route wired.
	body, _ = json.Marshal(map[string]string{"text": "go"})
	req = httptest.NewRequest("POST", "/terminal/inject", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/terminal/inject: expected 200, got %d", rec.Code)
	}
}
