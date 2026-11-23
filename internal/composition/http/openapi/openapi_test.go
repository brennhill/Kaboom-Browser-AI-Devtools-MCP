// openapi_test.go — Verifies the embedded OpenAPI document HTTP contract.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAPI(t *testing.T) {
	t.Parallel()
	handler := OpenAPI([]byte(`{"openapi":"3.0.3"}`))

	badResponse := httptest.NewRecorder()
	handler(badResponse, httptest.NewRequest(http.MethodPost, "/openapi.json", nil))
	if badResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", badResponse.Code, http.StatusMethodNotAllowed)
	}

	okResponse := httptest.NewRecorder()
	handler(okResponse, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if okResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", okResponse.Code, http.StatusOK)
	}
	if got := okResponse.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if got := okResponse.Body.String(); got != `{"openapi":"3.0.3"}` {
		t.Fatalf("body = %q", got)
	}
}
