// response_test.go — Tests JSON HTTP response helpers.
package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONResponse(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	JSONResponse(rr, http.StatusCreated, map[string]any{"ok": true, "count": 2})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("body ok = %v, want true", body["ok"])
	}
}

func TestJSONResponseEncodeErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	JSONResponse(rr, http.StatusOK, map[string]any{
		"bad": make(chan int), // unsupported by encoding/json
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}
