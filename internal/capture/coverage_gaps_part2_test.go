// Purpose: Coverage-expansion tests for capture pipeline edge cases and branch paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

// coverage_gaps_part2_test.go — Targeted tests for uncovered capture paths (part 2).
// Covers: AddExtensionLogs eviction, GetAll* empty branches, HandleRecordingStorage,
// HandleQueryResult correlation_id path, and accessor empty-slice branches.
package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// AddExtensionLogs — zero timestamp + eviction
// ============================================

func TestAddExtensionLogs_ZeroTimestamp(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	logs := []types.ExtensionLog{
		{Message: "test1", Source: "background", Category: "debug"},
	}
	c.ExtensionLogs().Add(logs)

	result := c.ExtensionLogs().Entries()
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0].Timestamp.IsZero() {
		t.Error("expected zero timestamp to be filled by AddExtensionLogs")
	}
	if result[0].Message != "test1" {
		t.Errorf("Message = %q, want test1", result[0].Message)
	}
}

func TestAddExtensionLogs_Eviction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2

	// Fill beyond eviction threshold (750) to trigger compaction.
	// After compaction, the buffer is trimmed to MaxExtensionLogs (500),
	// then remaining batch items are appended, so final count is
	// between MaxExtensionLogs and evictionThreshold.
	batch := make([]types.ExtensionLog, evictionThreshold+10)
	for i := range batch {
		batch[i] = types.ExtensionLog{
			Message:   "log entry",
			Source:    "background",
			Category:  "debug",
			Timestamp: time.Now(),
		}
	}
	c.ExtensionLogs().Add(batch)

	result := c.ExtensionLogs().Entries()
	if len(result) > evictionThreshold {
		t.Errorf("len = %d, should be at most evictionThreshold=%d", len(result), evictionThreshold)
	}
	if len(result) < MaxExtensionLogs {
		t.Errorf("len = %d, should be at least MaxExtensionLogs=%d", len(result), MaxExtensionLogs)
	}
}

// ============================================
// GetAllWebSocketEvents / GetAllEnhancedActions — empty branches
// ============================================

func TestGetAllWebSocketEvents_EmptyReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	result := c.Telemetry().GetAllWebSocketEvents()
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestGetAllEnhancedActions_EmptyReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	result := c.Telemetry().GetAllEnhancedActions()
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestGetNetworkBodies_EmptyReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	result := c.Telemetry().GetNetworkBodies()
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

// ============================================
// HandleRecordingStorage — GET (handleStorageGet)
// ============================================

func TestHandleRecordingStorage_GET(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/recording-storage", nil)
	c.HandleRecordingStorage(rr, req)

	// Should succeed (or return 500 if no recording directory configured — both are valid paths)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 200 or 500", rr.Code)
	}
}

func TestHandleRecordingStorage_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/recording-storage", nil)
	c.HandleRecordingStorage(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// ============================================
// HandleRecordingStorage — POST (handleStorageRecalculate)
// ============================================

func TestHandleRecordingStorage_POST(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/recording-storage", nil)
	c.HandleRecordingStorage(rr, req)

	// Should succeed (200) or fail if no recording directory (500)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 200 or 500", rr.Code)
	}
}

// ============================================
// HandleRecordingStorage — DELETE (handleStorageDelete)
// ============================================

func TestHandleRecordingStorage_DELETE_MissingID(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/recording-storage", nil)
	c.HandleRecordingStorage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected error message for missing recording_id")
	}
}

func TestHandleRecordingStorage_DELETE_NotFound(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/recording-storage?recording_id=nonexistent-id", nil)
	c.HandleRecordingStorage(rr, req)

	// Should return 404 for non-existent recording
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ============================================
// HandleQueryResult — correlation_id path
// ============================================

func TestHandleQueryResult_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodGet, "/query-result", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleQueryResult_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	rr := httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader("{bad")))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleQueryResult_WithCorrelationID(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	// Register a pending command with a known correlation ID
	corrID := "test-corr-id-001"
	c.Queries().RegisterCommand(corrID, "", 30*time.Second)

	payload := `{"correlation_id":"` + corrID + `","status":"complete","result":{"value":2}}`
	rr := httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(payload)))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("response status = %v, want ok", resp["status"])
	}
}

func TestHandleQueryResult_WithQueryID(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	payload := `{"id":"test-query-1","status":"complete","result":{"data":"hello"}}`
	rr := httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(payload)))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleQueryResult_WithCorrelationID_ErrorStatus(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	corrID := "test-corr-id-error-001"
	c.Queries().RegisterCommand(corrID, "", 30*time.Second)

	payload := `{"correlation_id":"` + corrID + `","status":"error","error":"boom"}`
	rr := runQueryResultRequest(t, c, payload)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	assertCommandResult(t, c, corrID, "error", "boom")
}

func TestHandleQueryResult_WithIDAndCorrelationID_PreservesErrorStatus(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	corrID := "test-corr-id-error-with-id-001"
	queryID, _ := c.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom_action",
		Params:        json.RawMessage(`{"action":"click","selector":"#publish"}`),
		CorrelationID: corrID,
	}, 30*time.Second, "")
	if queryID == "" {
		t.Fatal("expected query ID from CreatePendingQueryWithTimeout")
	}

	payload := `{"id":"` + queryID + `","correlation_id":"` + corrID + `","status":"error","error":"boom","result":{"success":false}}`
	rr := runQueryResultRequest(t, c, payload)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	assertCommandResult(t, c, corrID, "error", "boom")
}

// ============================================
// GetExtensionLogs — empty returns empty
// ============================================

func TestGetExtensionLogs_EmptyReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	result := c.ExtensionLogs().Entries()
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}
