// interact_storage_test.go — Tests for storage/cookie mutation handlers.
package toolinteract

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleSetStorage_Success(t *testing.T) {
	h, fs := newFakeStorageActions(t)
	resp := h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSetStorage_InvalidType(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"cookies","key":"k","value":"v"}`)), mcp.ErrInvalidParam)
}

func TestHandleSetStorage_MissingKey(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","value":"v"}`)), mcp.ErrMissingParam)
}

func TestHandleSetStorage_MissingValue(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k"}`)), mcp.ErrMissingParam)
}

func TestHandleSetStorage_InvalidJSON(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleSetStorage_InvalidWorld(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v","world":"moon"}`)), mcp.ErrInvalidParam)
}

func TestHandleDeleteStorage_Success(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"sessionStorage","key":"k"}`)))
}

func TestHandleDeleteStorage_MissingKey(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"sessionStorage"}`)), mcp.ErrMissingParam)
}

func TestHandleDeleteStorage_InvalidType(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"bogus","key":"k"}`)), mcp.ErrInvalidParam)
}

func TestHandleClearStorage_Success(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleClearStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage"}`)))
}

func TestHandleClearStorage_InvalidType(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleClearStorage(testReq(), json.RawMessage(`{"storage_type":"bogus"}`)), mcp.ErrInvalidParam)
}

func TestHandleClearStorage_InvalidJSON(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleClearStorage(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestStorageAndCookieActionsPreserveSharedExecutionTarget(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		run  func(*StorageActions, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	}{
		{
			name: "set",
			args: json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleSetStorage,
		},
		{
			name: "delete",
			args: json.RawMessage(`{"storage_type":"localStorage","key":"k","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleDeleteStorage,
		},
		{
			name: "clear",
			args: json.RawMessage(`{"storage_type":"localStorage","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleClearStorage,
		},
		{
			name: "set cookie",
			args: json.RawMessage(`{"name":"sid","value":"abc","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleSetCookie,
		},
		{
			name: "delete cookie",
			args: json.RawMessage(`{"name":"sid","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleDeleteCookie,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, fs := newFakeStorageActions(t)
			assertOK(t, tt.run(h, testReq(), tt.args))

			queued := fs.enqueuedSnapshot()
			if len(queued) != 1 {
				t.Fatalf("queued commands = %d, want 1", len(queued))
			}
			if queued[0].TabID != 42 {
				t.Fatalf("tab_id = %d, want 42", queued[0].TabID)
			}
			var params map[string]any
			if err := json.Unmarshal(queued[0].Params, &params); err != nil {
				t.Fatalf("decode queued params: %v", err)
			}
			if params["timeout_ms"] != float64(1234) {
				t.Fatalf("timeout_ms = %v, want 1234", params["timeout_ms"])
			}
			if params["world"] != "isolated" {
				t.Fatalf("world = %v, want isolated", params["world"])
			}
		})
	}
}

func TestHandleSetCookie_Success(t *testing.T) {
	h, fs := newFakeStorageActions(t)
	assertOK(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid","value":"abc","domain":"example.com","path":"/app"}`)))
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSetCookie_DefaultPath(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid","value":"abc"}`)))
}

func TestHandleSetCookie_MissingName(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"value":"abc"}`)), mcp.ErrMissingParam)
}

func TestHandleSetCookie_MissingValue(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid"}`)), mcp.ErrMissingParam)
}

func TestHandleDeleteCookie_Success(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{"name":"sid","domain":"example.com","path":"/app"}`)))
}

func TestHandleDeleteCookie_DefaultPath(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{"name":"sid"}`)))
}

func TestHandleDeleteCookie_MissingName(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestValidateStorageType(t *testing.T) {
	expr, _, ok := validateStorageType(testReq(), "localStorage")
	if !ok || expr != "localStorage" {
		t.Fatalf("expected localStorage valid, got %q ok=%v", expr, ok)
	}
	if _, _, ok := validateStorageType(testReq(), "nope"); ok {
		t.Fatal("expected invalid type rejected")
	}
}

func TestJSQuote(t *testing.T) {
	if jsQuote("a\"b") != `"a\"b"` {
		t.Fatalf("unexpected jsQuote output: %s", jsQuote("a\"b"))
	}
}
