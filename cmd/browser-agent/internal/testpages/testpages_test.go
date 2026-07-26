package testpages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleTestPagesSpecialEndpoints(t *testing.T) {
	t.Parallel()

	h := Handler()
	tests := []struct {
		path        string
		wantStatus  int
		wantBodySub string
	}{
		{path: "/tests/", wantStatus: http.StatusOK, wantBodySub: "Kaboom"},
		{path: "/tests/404", wantStatus: http.StatusNotFound, wantBodySub: "network_error"},
		{path: "/tests/500", wantStatus: http.StatusInternalServerError, wantBodySub: "network_error"},
		{path: "/tests/cors-test", wantStatus: http.StatusOK, wantBodySub: "cors_block"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			h.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status mismatch: got %d want %d", rr.Code, tt.wantStatus)
			}
			if !strings.Contains(rr.Body.String(), tt.wantBodySub) {
				t.Fatalf("body missing %q: %q", tt.wantBodySub, rr.Body.String())
			}
		})
	}
}

func TestHandleTestPagesMethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := Handler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tests/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusMethodNotAllowed)
	}
	if !strings.Contains(rr.Body.String(), "Method not allowed") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleTestHarnessWSValidation(t *testing.T) {
	t.Parallel()

	t.Run("missing upgrade headers", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tests/ws", nil)
		HandlerWS(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusBadRequest)
		}
		if !strings.Contains(rr.Body.String(), "websocket upgrade required") {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})

	t.Run("upgrade requested but writer is not hijacker", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tests/ws", nil)
		req.Header.Set("Sec-WebSocket-Key", "abc")
		req.Header.Set("Upgrade", "websocket")
		HandlerWS(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(rr.Body.String(), "does not support hijacking") {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})
}
