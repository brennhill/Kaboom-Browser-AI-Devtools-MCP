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

// Every endpoint used to accept any well-formed JSON object. encoding/json
// ignores unrecognized fields, so an error envelope decoded into a zero-valued
// request and HandleStart opened a trace for tab 0 and answered 201. The reply
// is what the extension retries against, so the failure was invisible on both
// sides.
func TestHTTPRejectsBodiesSharingNoFieldWithTheRequestType(t *testing.T) {
	unrecognized := []struct {
		name string
		body string
	}{
		{"in-band error envelope", `{"error":"trace_start_failed","message":"debugger already attached"}`},
		{"a response shape sent to a request endpoint", `{"trace_id_typo":"t-1","recovered":false}`},
		{"an unrelated object", `{"status":"queued","correlation_id":"abc"}`},
	}
	for _, payload := range unrecognized {
		t.Run(payload.name, func(t *testing.T) {
			h := NewHTTPHandler(NewManager(t.TempDir()))
			got := postTraceJSON(t, h.HandleStart, payload.body)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("status = %d (body %s), want 400 — an unrecognized body must not open a trace",
					got.Code, got.Body.String())
			}
			// Checked as a decoded field rather than a substring: the
			// rejection message quotes the offending keys back, so a body
			// naming trace_id_typo legitimately contains "trace_id".
			var reply map[string]json.RawMessage
			if err := json.Unmarshal(got.Body.Bytes(), &reply); err != nil {
				t.Fatalf("rejection body was not JSON: %s", got.Body.String())
			}
			if _, opened := reply["trace_id"]; opened {
				t.Errorf("body = %s, want no trace handed back for a rejected request", got.Body.String())
			}
			if _, reported := reply["error"]; !reported {
				t.Errorf("body = %s, want the rejection to say why", got.Body.String())
			}
		})
	}
}

// The mirror of the case above: a body the endpoint does understand must still
// be accepted when it also carries fields this build has never heard of, or
// every extension release ahead of the daemon becomes an outage.
func TestHTTPAcceptsKnownFieldsAlongsideUnknownOnes(t *testing.T) {
	h := NewHTTPHandler(NewManager(t.TempDir()))
	got := postTraceJSON(t, h.HandleStart, `{"tab_id":12,"field_from_a_newer_extension":true}`)
	if got.Code != http.StatusCreated {
		t.Fatalf("status = %d (body %s), want 201 — unknown fields must stay forward compatible",
			got.Code, got.Body.String())
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
