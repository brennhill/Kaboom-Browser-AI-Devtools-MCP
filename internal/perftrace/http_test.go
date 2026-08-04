// http_test.go — Verifies the local performance trace upload HTTP contract.

package perftrace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPLifecycle(t *testing.T) {
	h := NewHTTPHandler(NewManager(t.TempDir()))

	start := postTraceJSON(t, h.HandleStart, `{"tab_id":12}`)
	if start.Code != http.StatusCreated {
		t.Fatalf("start status = %d, body=%s", start.Code, start.Body.String())
	}
	var opened WirePerformanceTraceStartResponse
	if err := json.Unmarshal(start.Body.Bytes(), &opened); err != nil {
		t.Fatal(err)
	}

	chunk := postTraceJSON(t, h.HandleChunk, `{"trace_id":"`+opened.TraceID+`","sequence":0,"events":[{"name":"RunTask"}]}`)
	if chunk.Code != http.StatusAccepted {
		t.Fatalf("chunk status = %d, body=%s", chunk.Code, chunk.Body.String())
	}
	finish := postTraceJSON(t, h.HandleFinish, `{"trace_id":"`+opened.TraceID+`"}`)
	if finish.Code != http.StatusOK {
		t.Fatalf("finish status = %d, body=%s", finish.Code, finish.Body.String())
	}
}

func TestHTTPRejectsWrongMethodAndOversizedBody(t *testing.T) {
	h := NewHTTPHandler(NewManager(t.TempDir()))
	w := httptest.NewRecorder()
	h.HandleStart(w, httptest.NewRequest(http.MethodGet, "/performance-trace/start", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d", w.Code)
	}

	w = httptest.NewRecorder()
	body := strings.NewReader(`{"trace_id":"x","sequence":0,"events":[],"padding":"` + strings.Repeat("x", int(maxChunkBodyBytes)) + `"}`)
	h.HandleChunk(w, httptest.NewRequest(http.MethodPost, "/performance-trace/chunk", body))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d", w.Code)
	}
}

func postTraceJSON(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler(w, req)
	return w
}
