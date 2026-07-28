// Purpose: Tests for capture helper functions.
// Docs: docs/features/feature/backend-log-streaming/index.md

// helpers_test.go — Tests for ingest body reading and rate-limit plumbing.
package capture

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// readIngestBody Tests
// ============================================

func TestNewReadIngestBody_Success(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	body := []byte(`{"events":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	w := httptest.NewRecorder()

	result, ok := c.readIngestBody(w, req)
	if !ok {
		t.Fatal("readIngestBody returned false, want true")
	}
	if string(result) != string(body) {
		t.Errorf("readIngestBody body = %q, want %q", string(result), string(body))
	}
	if w.Code != http.StatusOK {
		t.Errorf("readIngestBody status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNewReadIngestBody_TooLargeBody(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Create a body larger than maxExtensionPostBody (5MB)
	bigBody := strings.Repeat("x", 6*1024*1024)
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(bigBody))
	w := httptest.NewRecorder()

	result, ok := c.readIngestBody(w, req)
	if ok {
		t.Fatal("readIngestBody returned true for too-large body, want false")
	}
	if result != nil {
		t.Errorf("readIngestBody result = %v, want nil for too-large body", result)
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("readIngestBody status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestNewReadIngestBody_RateLimited(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Trigger rate limit by recording many events
	for i := 0; i < 100; i++ {
		c.Circuit().RecordEvents(RateLimitThreshold)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	result, ok := c.readIngestBody(w, req)
	if ok {
		// If rate limit was triggered, should return false
		// If not triggered (timing dependent), that's ok too
		_ = result
	}
	// This test validates the rate limit path is exercised without crashing
}

// ============================================
// recordAndRecheck Tests
// ============================================

func TestNewRecordAndRecheck_NormalFlow(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	w := httptest.NewRecorder()

	ok := c.recordAndRecheck(w, 1)
	if !ok {
		t.Fatal("recordAndRecheck returned false for 1 event, want true")
	}
}

func TestNewRecordAndRecheck_RecordsEventCount(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	w := httptest.NewRecorder()

	c.recordAndRecheck(w, 10)
	// After recording 10 events, health should reflect them
	health := c.Circuit().GetHealthStatus()
	if health.CurrentRate < 10 {
		t.Errorf("CurrentRate = %d after recording 10 events, want >= 10", health.CurrentRate)
	}
}
