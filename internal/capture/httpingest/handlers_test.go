// handlers_test.go — Verifies the capture HTTP ingestion boundary.

package httpingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
)

func TestNetworkBodiesRejectsInvalidJSONWithoutMutatingTelemetry(t *testing.T) {
	t.Parallel()

	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/network-bodies", strings.NewReader("{"))

	New(dependencies(captured)).HandleNetworkBodies(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := captured.Telemetry().NetworkBodies().Stats().Count; got != 0 {
		t.Fatalf("retained bodies = %d, want 0", got)
	}
}

func TestIngestBodyEnforcesSizeAndRateLimits(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	handler := New(dependencies(captured))

	recorder := httptest.NewRecorder()
	body, ok := handler.readIngestBody(recorder, httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(`{"events":[]}`)))
	if !ok || string(body) != `{"events":[]}` {
		t.Fatalf("normal ingest = %q/%v", body, ok)
	}

	recorder = httptest.NewRecorder()
	_, ok = handler.readIngestBody(recorder, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(strings.Repeat("x", maxExtensionPostBody+1))))
	if ok || recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized ingest status/ok = %d/%v", recorder.Code, ok)
	}

	for range 100 {
		captured.Circuit().RecordEvents(circuit.RateLimitThreshold)
	}
	recorder = httptest.NewRecorder()
	_, ok = handler.readIngestBody(recorder, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{}`)))
	if ok || recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited ingest status/ok = %d/%v", recorder.Code, ok)
	}
}

func TestRecordingStorageGetAndRecalculate(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	handler := New(dependencies(captured))

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		recorder := httptest.NewRecorder()
		handler.HandleRecordingStorage(recorder, httptest.NewRequest(method, "/recordings/storage", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", method, recorder.Code)
		}
		var response map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("%s invalid JSON: %v", method, err)
		}
	}
}

func TestNetworkBodiesIngestsThroughCanonicalTelemetryOwner(t *testing.T) {
	t.Parallel()

	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/network-bodies", strings.NewReader(`{"bodies":[{"url":"https://example.test","status":200}]}`))

	New(dependencies(captured)).HandleNetworkBodies(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := captured.Telemetry().NetworkBodies().Stats().Count; got != 1 {
		t.Fatalf("retained bodies = %d, want 1", got)
	}
}

func dependencies(captured *capture.Capture) Dependencies {
	return Dependencies{Telemetry: captured.Telemetry(), Queries: captured.Queries(), Recordings: captured.Recordings(), Performance: captured.Performance(), Circuit: captured.Circuit()}
}
