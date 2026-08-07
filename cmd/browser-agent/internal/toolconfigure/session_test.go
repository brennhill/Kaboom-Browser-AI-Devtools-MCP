// session_test.go — Tests configure session persistence entry points.

package toolconfigure

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

func newSessionTestHandler(t *testing.T, deps SessionDeps) *SessionHandler {
	t.Helper()
	store, err := persistence.NewSessionStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	if deps.RequireStore == nil {
		deps.RequireStore = func(mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			return mcp.JSONRPCResponse{}, false
		}
	}
	return NewSessionHandler(deps, store, nil)
}

func TestSessionHandlerLoadReportsPersistedContext(t *testing.T) {
	t.Parallel()
	store, err := persistence.NewSessionStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	handler := NewSessionHandler(SessionDeps{}, store, nil)
	result := parseRespJSON(t, handler.Load(newReq(), nil))
	if result["status"] != "ok" || result["project_id"] == nil || result["session_count"] == nil {
		t.Fatalf("loaded session context = %#v", result)
	}
}

func TestSessionHandlerLoadFailsClearlyWithoutStore(t *testing.T) {
	t.Parallel()
	handler := NewSessionHandler(SessionDeps{}, nil, nil)
	isError, text := parseResp(t, handler.Load(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, nil))
	if !isError || (!strings.Contains(text, mcp.ErrNotInitialized) && !strings.Contains(text, "not initialized")) {
		t.Fatalf("nil-store response = error:%t text:%q", isError, text)
	}
}

func TestSessionHandlerStoreDefaultsToSessionList(t *testing.T) {
	t.Parallel()
	handler := newSessionTestHandler(t, SessionDeps{})
	result := parseRespJSON(t, handler.Store(newReq(), json.RawMessage(`{}`)))
	if result["namespace"] != "session" || result["keys"] == nil {
		t.Fatalf("default store list = %#v", result)
	}
}

func TestSessionHandlerStoreSavesAndLoadsCanonicalData(t *testing.T) {
	t.Parallel()
	var invalidated bool
	var activeCodebase string
	handler := newSessionTestHandler(t, SessionDeps{
		InvalidateSummary: func() { invalidated = true },
		SetActiveCodebase: func(path string) { activeCodebase = path },
	})

	saved := parseRespJSON(t, handler.Store(newReq(), json.RawMessage(
		`{"store_action":"save","key":"flat_key","data":"flat_value"}`,
	)))
	if saved["status"] != "saved" || saved["namespace"] != "session" {
		t.Fatalf("save response = %#v", saved)
	}
	loaded := parseRespJSON(t, handler.Store(newReq(), json.RawMessage(
		`{"store_action":"load","key":"flat_key"}`,
	)))
	if loaded["namespace"] != "session" || loaded["key"] != "flat_key" || loaded["data"] != "flat_value" {
		t.Fatalf("load response = %#v", loaded)
	}
	_ = handler.Store(newReq(), json.RawMessage(
		`{"store_action":"save","key":"response_mode","data":"compact"}`,
	))
	_ = handler.Store(newReq(), json.RawMessage(
		`{"store_action":"save","key":"active_codebase","data":"/tmp/project"}`,
	))
	if !invalidated || activeCodebase != "/tmp/project" {
		t.Fatalf("store side effects = invalidated:%t active:%q", invalidated, activeCodebase)
	}
}

func TestSessionHandlerStoreRejectsMalformedArguments(t *testing.T) {
	t.Parallel()
	handler := newSessionTestHandler(t, SessionDeps{})
	isError, text := parseResp(t, handler.Store(newReq(), json.RawMessage(`{bad}`)))
	if !isError || !strings.Contains(text, mcp.ErrInvalidJSON) {
		t.Fatalf("malformed store response = error:%t text:%q", isError, text)
	}
}
