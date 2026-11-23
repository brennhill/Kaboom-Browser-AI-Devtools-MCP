// stats_endpoint_test.go — Tests for the token savings HTTP endpoint.

package tracking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRecordTokenSavings_ValidRequest(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()
	handler := HandleRecordTokenSavings(tr)

	body := `{"category":"test_output","tokens_before":2000,"tokens_after":50}`
	req := httptest.NewRequest(http.MethodPost, "/api/token-savings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var stats SessionStats
	if err := json.NewDecoder(rr.Body).Decode(&stats); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if stats.TotalTokensSaved != 1950 {
		t.Errorf("TotalTokensSaved = %d, want 1950", stats.TotalTokensSaved)
	}
	if stats.TotalCompressions != 1 {
		t.Errorf("TotalCompressions = %d, want 1", stats.TotalCompressions)
	}
}

func TestHandleRecordTokenSavings_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()
	handler := HandleRecordTokenSavings(tr)

	req := httptest.NewRequest(http.MethodGet, "/api/token-savings", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleRecordTokenSavings_InvalidJSON(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()
	handler := HandleRecordTokenSavings(tr)

	req := httptest.NewRequest(http.MethodPost, "/api/token-savings", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRecordTokenSavings_MissingCategory(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()
	handler := HandleRecordTokenSavings(tr)

	body := `{"tokens_before":2000,"tokens_after":50}`
	req := httptest.NewRequest(http.MethodPost, "/api/token-savings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRecordTokenSavings_InvalidTokensBefore(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()
	handler := HandleRecordTokenSavings(tr)

	body := `{"category":"test_output","tokens_before":0,"tokens_after":50}`
	req := httptest.NewRequest(http.MethodPost, "/api/token-savings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRecordTokenSavings_MultipleRequests(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()
	handler := HandleRecordTokenSavings(tr)

	// First request.
	body1 := `{"category":"test_output","tokens_before":2000,"tokens_after":50}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/token-savings", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	// Second request.
	body2 := `{"category":"build_output","tokens_before":1000,"tokens_after":100}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/token-savings", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	var stats SessionStats
	if err := json.NewDecoder(rr2.Body).Decode(&stats); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if stats.TotalTokensSaved != 2850 {
		t.Errorf("TotalTokensSaved = %d, want 2850", stats.TotalTokensSaved)
	}
	if stats.TotalCompressions != 2 {
		t.Errorf("TotalCompressions = %d, want 2", stats.TotalCompressions)
	}
}
