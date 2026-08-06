// handlers_logging_test.go -- Regression: state-mutating terminal failures must
// be logged (structured) on the daemon, not only returned to the client.

package terminal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// depsWithLogCapture returns test Deps plus a thread-safe recorder of logEvent
// calls, so tests can assert a structured event was emitted.
func depsWithLogCapture() (Deps, func(event string) map[string]any) {
	var mu sync.Mutex
	events := map[string]map[string]any{}
	deps := testDeps()
	deps.LogEvent = func(event string, fields map[string]any) {
		mu.Lock()
		events[event] = fields
		mu.Unlock()
	}
	return deps, func(event string) map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return events[event]
	}
}

func TestHandleTerminalStart_LogsFailure(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps, get := depsWithLogCapture()

	// A non-existent cwd makes the spawn fail — a state-mutating failure that
	// must be logged, not just returned to the client.
	body, _ := json.Marshal(map[string]any{
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
		"dir":  "/no/such/kaboom-dir-xyz-does-not-exist",
	})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, sessionrelay.NewMap())

	if rec.Code < 400 {
		t.Fatalf("expected a failure status, got %d", rec.Code)
	}
	ev := get("terminal_session_start_failed")
	if ev == nil {
		t.Fatal("expected a structured terminal_session_start_failed event to be logged")
	}
	if ev["error"] == nil || ev["error"] == "" {
		t.Fatalf("failure event must carry the underlying error, got %v", ev["error"])
	}
	if ev["dir"] != "/no/such/kaboom-dir-xyz-does-not-exist" {
		t.Fatalf("failure event should record the attempted dir, got %v", ev["dir"])
	}
}

func TestHandleTerminalStart_SuccessDoesNotLogFailure(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps, get := depsWithLogCapture()

	body, _ := json.Marshal(map[string]any{"cmd": "/bin/sh", "args": []string{"-c", "exec cat"}})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, sessionrelay.NewMap())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ev := get("terminal_session_start_failed"); ev != nil {
		t.Fatalf("a successful start must not log a failure event, got %v", ev)
	}
}

// Regression for "terminal never recovers": after a session's child exits on its
// own, the NEXT Start must self-heal (200 + a fresh token + a healed event), not
// return 409 with the dead session's stale token (which stranded the terminal on a
// closed fanout forever).
func TestHandleTerminalStart_SelfHealsDeadSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps, get := depsWithLogCapture()
	relays := sessionrelay.NewMap()

	start := func(cmd string, args []string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"cmd": cmd, "args": args})
		req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		HandleTerminalStart(rec, req, deps, nil, mgr, nil, relays)
		return rec
	}
	tokenOf := func(rec *httptest.ResponseRecorder) string {
		var resp struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return resp.Token
	}

	// First start: a shell that exits immediately.
	rec1 := start("/bin/sh", []string{"-c", "exit 0"})
	if rec1.Code != http.StatusOK {
		t.Fatalf("first start: expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}
	token1 := tokenOf(rec1)

	// Wait for the child to die so the session is no longer alive.
	sess, err := mgr.Get("default")
	if err != nil {
		t.Fatalf("Get(default): %v", err)
	}
	_ = sess.Wait(3 * time.Second)
	if sess.IsAlive() {
		t.Fatal("child should have exited")
	}

	// Second start of the same (default) id must self-heal, not 409.
	rec2 := start("/bin/sh", []string{"-c", "exec cat"})
	if rec2.Code != http.StatusOK {
		t.Fatalf("self-heal start: expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	token2 := tokenOf(rec2)
	if token2 == "" || token2 == token1 {
		t.Fatalf("self-heal must mint a fresh token (token1=%q token2=%q)", token1, token2)
	}
	if ev := get("terminal_session_healed"); ev == nil {
		t.Fatal("expected a structured terminal_session_healed event on self-heal")
	}
	// The new session must be reachable by its fresh token (relay rebuilt).
	if _, err := mgr.GetByToken(token2); err != nil {
		t.Fatalf("new token should resolve to a live session, got %v", err)
	}
}

func TestHandleTerminalStop_LogsFailure(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps, get := depsWithLogCapture()

	// Stop a session that does not exist -> mgr.Stop errors -> logged failure.
	body, _ := json.Marshal(map[string]any{"id": "no-such-session"})
	req := httptest.NewRequest("POST", "/terminal/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleTerminalStop(rec, req, deps, mgr, sessionrelay.NewMap())

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown session, got %d", rec.Code)
	}
	if ev := get("terminal_session_stop_failed"); ev == nil {
		t.Fatal("expected a structured terminal_session_stop_failed event to be logged")
	}
}
