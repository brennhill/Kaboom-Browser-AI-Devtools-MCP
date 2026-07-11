// interact_storage_test.go — Tests for storage/cookie mutation handlers.
package interacthandler

import (
	"encoding/json"
	"testing"
)

func TestHandleSetStorage_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	resp := h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSetStorage_InvalidType(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"cookies","key":"k","value":"v"}`)), ErrInvalidParam)
}

func TestHandleSetStorage_MissingKey(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","value":"v"}`)), ErrMissingParam)
}

func TestHandleSetStorage_MissingValue(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k"}`)), ErrMissingParam)
}

func TestHandleSetStorage_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleSetStorage_InvalidWorld(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v","world":"moon"}`)), ErrInvalidParam)
}

func TestHandleDeleteStorage_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"sessionStorage","key":"k"}`)))
}

func TestHandleDeleteStorage_MissingKey(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"sessionStorage"}`)), ErrMissingParam)
}

func TestHandleDeleteStorage_InvalidType(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"bogus","key":"k"}`)), ErrInvalidParam)
}

func TestHandleClearStorage_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleClearStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage"}`)))
}

func TestHandleClearStorage_InvalidType(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleClearStorage(testReq(), json.RawMessage(`{"storage_type":"bogus"}`)), ErrInvalidParam)
}

func TestHandleClearStorage_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleClearStorage(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleSetCookie_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	assertOK(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid","value":"abc","domain":"example.com","path":"/app"}`)))
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSetCookie_DefaultPath(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid","value":"abc"}`)))
}

func TestHandleSetCookie_MissingName(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"value":"abc"}`)), ErrMissingParam)
}

func TestHandleSetCookie_MissingValue(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid"}`)), ErrMissingParam)
}

func TestHandleDeleteCookie_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{"name":"sid","domain":"example.com","path":"/app"}`)))
}

func TestHandleDeleteCookie_DefaultPath(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{"name":"sid"}`)))
}

func TestHandleDeleteCookie_MissingName(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{}`)), ErrMissingParam)
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
