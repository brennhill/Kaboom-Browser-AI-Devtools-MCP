// handlers_extra_test.go -- Tests for active-codebase config, CWD auto-detection,
// default-shell selection, control-message parsing, and route registration.

package terminal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestHandleActiveCodebase_Get(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	server := &fakeServerDeps{codebase: "/home/user/proj"}

	req := httptest.NewRequest("GET", "/config/active-codebase", nil)
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, req, deps, server)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["active_codebase"] != "/home/user/proj" {
		t.Fatalf("got %q, want /home/user/proj", resp["active_codebase"])
	}
}

func TestHandleActiveCodebase_SetPutAndPost(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"PUT", "POST"} {
		t.Run(method, func(t *testing.T) {
			deps := testDeps()
			server := &fakeServerDeps{}
			body, _ := json.Marshal(map[string]string{"path": "  /new/path  "})
			req := httptest.NewRequest(method, "/config/active-codebase", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			HandleActiveCodebase(rec, req, deps, server)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			// Path should be trimmed before storing.
			if server.codebase != "/new/path" {
				t.Fatalf("stored codebase = %q, want /new/path (trimmed)", server.codebase)
			}
			var resp map[string]string
			_ = json.NewDecoder(rec.Body).Decode(&resp)
			if resp["status"] != "ok" || resp["active_codebase"] != "/new/path" {
				t.Fatalf("unexpected response: %+v", resp)
			}
		})
	}
}

func TestHandleActiveCodebase_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	req := httptest.NewRequest("PUT", "/config/active-codebase", strings.NewReader("{bad"))
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, req, deps, &fakeServerDeps{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleActiveCodebase_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	req := httptest.NewRequest("DELETE", "/config/active-codebase", nil)
	rec := httptest.NewRecorder()
	HandleActiveCodebase(rec, req, deps, &fakeServerDeps{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestAutoDetectCWD_NilRegistry(t *testing.T) {
	t.Parallel()
	// A fresh capture store has no client registry wired.
	store := capture.NewCapture()
	if got := AutoDetectCWD(store); got != "" {
		t.Fatalf("expected empty CWD with nil registry, got %q", got)
	}
}

func TestAutoDetectCWD_NilClientList(t *testing.T) {
	t.Parallel()
	store := capture.NewCapture()
	store.SetClientRegistry(&fakeClientRegistry{listResult: nil})
	if got := AutoDetectCWD(store); got != "" {
		t.Fatalf("expected empty CWD when List() is nil, got %q", got)
	}
}

func TestAutoDetectCWD_AnySliceBranch(t *testing.T) {
	t.Parallel()
	store := capture.NewCapture()
	store.SetClientRegistry(&fakeClientRegistry{listResult: []any{
		map[string]any{"cwd": ""},               // skipped: empty
		"not-a-map",                              // skipped: wrong type
		map[string]any{"other": "x"},            // skipped: no cwd
		map[string]any{"cwd": "/first/real/cwd"}, // taken
	}})
	if got := AutoDetectCWD(store); got != "/first/real/cwd" {
		t.Fatalf("got %q, want /first/real/cwd", got)
	}
}

func TestAutoDetectCWD_AnySliceNoCWD(t *testing.T) {
	t.Parallel()
	store := capture.NewCapture()
	store.SetClientRegistry(&fakeClientRegistry{listResult: []any{
		map[string]any{"cwd": ""},
		map[string]any{"other": "y"},
	}})
	if got := AutoDetectCWD(store); got != "" {
		t.Fatalf("expected empty when no client has cwd, got %q", got)
	}
}

func TestAutoDetectCWD_JSONRoundtripBranch(t *testing.T) {
	t.Parallel()
	// A typed slice (not []any) forces the default JSON-roundtrip branch.
	type clientInfo struct {
		CWD string `json:"cwd"`
	}
	store := capture.NewCapture()
	store.SetClientRegistry(&fakeClientRegistry{listResult: []clientInfo{
		{CWD: ""},
		{CWD: "/roundtrip/cwd"},
	}})
	if got := AutoDetectCWD(store); got != "/roundtrip/cwd" {
		t.Fatalf("got %q, want /roundtrip/cwd", got)
	}
}

func TestAutoDetectCWD_JSONRoundtripEmpty(t *testing.T) {
	t.Parallel()
	type clientInfo struct {
		CWD string `json:"cwd"`
	}
	store := capture.NewCapture()
	store.SetClientRegistry(&fakeClientRegistry{listResult: []clientInfo{{CWD: ""}}})
	if got := AutoDetectCWD(store); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestDefaultShell_HonorsValidSHELL(t *testing.T) {
	// Not parallel: mutates process env.
	// Pick a shell that exists on this host to use as SHELL.
	var real string
	for _, c := range []string{"/bin/sh", "/bin/bash", "/bin/zsh"} {
		if _, err := exec.LookPath(c); err == nil {
			real = c
			break
		}
	}
	if real == "" {
		t.Skip("no standard shell available on host")
	}
	t.Setenv("SHELL", real)
	if got := defaultShell(); got != real {
		t.Fatalf("defaultShell() = %q, want %q (from SHELL)", got, real)
	}
}

func TestDefaultShell_FallsBackWhenSHELLMissing(t *testing.T) {
	// Not parallel: mutates process env.
	t.Setenv("SHELL", "/nonexistent/definitely-not-a-shell")
	got := defaultShell()
	// Must fall back to a shell that exists (or the final /bin/sh default).
	if got == "/nonexistent/definitely-not-a-shell" {
		t.Fatal("defaultShell() returned the invalid SHELL value")
	}
	if _, err := os.Stat(got); err != nil && got != "/bin/sh" {
		t.Fatalf("defaultShell() = %q which does not exist", got)
	}
}

func TestDefaultShell_EmptySHELL(t *testing.T) {
	t.Setenv("SHELL", "")
	got := defaultShell()
	if got == "" {
		t.Fatal("defaultShell() returned empty string")
	}
}

func TestHandleControlMessage_NoSessionAccessPaths(t *testing.T) {
	t.Parallel()
	// These payloads must NOT dereference the session, so a nil session is safe.
	cases := []struct {
		name    string
		payload string
	}{
		{"invalid json", "{not json"},
		{"unknown type", `{"type":"unknown"}`},
		{"resize zero cols", `{"type":"resize","cols":0,"rows":24}`},
		{"resize zero rows", `{"type":"resize","cols":80,"rows":0}`},
		{"resize negative", `{"type":"resize","cols":-1,"rows":-1}`},
		{"empty object", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic with nil session on these early-return paths.
			HandleControlMessage([]byte(tc.payload), nil)
		})
	}
}

func TestHandleControlMessage_ResizeAppliesToSession(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "resize-test", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get(res.SessionID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// Valid resize path: exercises Resize + ForceRedraw. Should not error/panic.
	HandleControlMessage([]byte(`{"type":"resize","cols":120,"rows":40}`), sess)
}

func TestRegisterRoutes_DispatchesWithoutSpawning(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	mgr := pty.NewManager()
	defer mgr.StopAll()
	store := capture.NewCapture()
	server := &fakeServerDeps{}

	mux := http.NewServeMux()
	relays := RegisterRoutes(mux, deps, server, mgr, store)
	if relays == nil {
		t.Fatal("RegisterRoutes returned nil relay map")
	}

	cases := []struct {
		name       string
		method     string
		target     string
		wantStatus int
	}{
		{"terminal page", "GET", "/terminal", http.StatusOK},
		{"static missing asset", "GET", "/terminal/static/does-not-exist.js", http.StatusNotFound},
		{"ws missing token", "GET", "/terminal/ws", http.StatusUnauthorized},
		{"start wrong method", "GET", "/terminal/start", http.StatusMethodNotAllowed},
		{"stop nonexistent", "POST", "/terminal/stop", http.StatusNotFound},
		{"validate empty token", "GET", "/terminal/validate", http.StatusOK},
		{"config list", "GET", "/terminal/config", http.StatusOK},
		{"upload wrong method", "GET", "/terminal/upload", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reqBody *bytes.Reader
			if tc.method == "POST" {
				b, _ := json.Marshal(map[string]string{"id": "nonexistent"})
				reqBody = bytes.NewReader(b)
			} else {
				reqBody = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.target, reqBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("%s %s: got %d, want %d (%s)", tc.method, tc.target, rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}
