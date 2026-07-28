// Purpose: Coverage-expansion tests for capture pipeline edge cases and branch paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func newCoverageCapture(t *testing.T) *Capture {
	t.Helper()
	c := NewCapture()
	t.Cleanup(c.Close)
	return c
}

func TestCoverageBoost_SetupHelpers(t *testing.T) {
	c := setupTestCapture(t)
	if c == nil {
		t.Fatal("setupTestCapture returned nil")
	}
	c.Close()

	srv, logFile := setupTestServer(t)
	if srv == nil {
		t.Fatal("setupTestServer returned nil server")
	}
	if logFile == "" {
		t.Fatal("setupTestServer returned empty log file path")
	}
	if _, err := os.Stat(filepath.Dir(logFile)); err != nil {
		t.Fatalf("setupTestServer log dir stat error = %v", err)
	}

	if got := setupToolHandler(t, srv, NewCapture()); got != nil {
		t.Fatalf("setupToolHandler() = %v, want nil placeholder", got)
	}
}

func TestCoverageBoost_RateLimitHealthHandler(t *testing.T) {
	c := newCoverageCapture(t)
	c.RecordEvents(42)

	health := c.GetHealthStatus()
	if health.CurrentRate < 42 {
		t.Fatalf("CurrentRate = %d, want at least 42", health.CurrentRate)
	}

	rrBad := httptest.NewRecorder()
	reqBad := httptest.NewRequest(http.MethodPost, "/health", nil)
	c.HandleHealth(rrBad, reqBad)
	if rrBad.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status = %d, want %d", rrBad.Code, http.StatusMethodNotAllowed)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	c.HandleHealth(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", rr.Code, http.StatusOK)
	}
	var got circuit.HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal health response error = %v", err)
	}
	if got.CurrentRate != health.CurrentRate {
		t.Fatalf("health current_rate = %d, want %d", got.CurrentRate, health.CurrentRate)
	}
}

func TestCoverageBoost_PublicMemoryAndBufferGetters(t *testing.T) {
	c := newCoverageCapture(t)

	c.AddWebSocketEvents([]types.WebSocketEvent{{
		ID:        "conn-1",
		Event:     "message",
		Direction: "incoming",
		Data:      "hello",
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}})
	c.AddNetworkBodies([]types.NetworkBody{{
		Method:       "POST",
		URL:          "https://example.test/api",
		Status:       200,
		RequestBody:  "abc",
		ResponseBody: "def",
	}})

	if got := c.buffers.calcWSMemory(); got <= 0 {
		t.Fatalf("GetWebSocketBufferMemory() = %d, want > 0", got)
	}
	if got := c.buffers.calcNBMemory(); got <= 0 {
		t.Fatalf("GetNetworkBodiesBufferMemory() = %d, want > 0", got)
	}
	if got := len(c.GetNetworkBodies()); got == 0 {
		t.Fatal("GetNetworkBodyCount() = 0, want > 0")
	}
}

func TestCoverageBoost_EnhancedActionsBranches(t *testing.T) {
	c := newCoverageCapture(t)

	c.mu.Lock()
	now := time.Now()
	c.buffers.enhancedActions = []enhancedActionEntry{
		{Action: types.EnhancedAction{Type: "click"}, AddedAt: now},
		{Action: types.EnhancedAction{Type: "click"}, AddedAt: now},
	}
	c.extensionState.activeTestIDs["test-1"] = true
	c.mu.Unlock()

	c.AddEnhancedActions([]types.EnhancedAction{{Type: "type", Value: "hello"}})
	if got := len(c.GetAllEnhancedActions()); got != 3 {
		t.Fatalf("GetEnhancedActionCount() = %d, want 3 after add", got)
	}

	actions := c.GetAllEnhancedActions()
	if len(actions) == 0 {
		t.Fatal("GetAllEnhancedActions() returned empty actions")
	}
	last := actions[len(actions)-1]
	if len(last.TestIDs) == 0 || last.TestIDs[0] != "test-1" {
		t.Fatalf("last action TestIDs = %+v, want [test-1]", last.TestIDs)
	}

	many := make([]types.EnhancedAction, MaxEnhancedActions+5)
	for i := range many {
		many[i] = types.EnhancedAction{Type: "click"}
	}
	c.AddEnhancedActions(many)
	if got := len(c.GetAllEnhancedActions()); got != MaxEnhancedActions {
		t.Fatalf("GetEnhancedActionCount() after rotation = %d, want %d", got, MaxEnhancedActions)
	}
}

func TestCoverageBoost_NetworkBodiesBranches(t *testing.T) {
	c := newCoverageCapture(t)

	now := time.Now()
	c.mu.Lock()
	c.buffers.networkBodies = []networkBodyEntry{
		{Body: types.NetworkBody{Method: "GET", URL: "https://a.example", RequestBody: "a", ResponseBody: "a"}, AddedAt: now},
		{Body: types.NetworkBody{Method: "GET", URL: "https://b.example", RequestBody: "b", ResponseBody: "b"}, AddedAt: now},
	}
	c.extensionState.activeTestIDs["tid"] = true
	c.mu.Unlock()

	c.AddNetworkBodies([]types.NetworkBody{{
		Method:       "POST",
		URL:          "https://example.test/upload",
		RequestBody:  "ping",
		ResponseBody: "pong",
	}})
	if got := len(c.GetNetworkBodies()); got != 3 {
		t.Fatalf("GetNetworkBodyCount() = %d, want 3 after add", got)
	}
	bodies := c.GetNetworkBodies()
	last := bodies[len(bodies)-1]
	if len(last.TestIDs) == 0 || last.TestIDs[0] != "tid" {
		t.Fatalf("last network body TestIDs = %+v, want [tid]", last.TestIDs)
	}

	c2 := newCoverageCapture(t)
	huge := strings.Repeat("x", nbBufferMemoryLimit)
	c2.AddNetworkBodies([]types.NetworkBody{{
		Method:       "POST",
		URL:          "https://example.test/huge",
		RequestBody:  huge,
		ResponseBody: huge,
	}})
	if got := len(c2.GetNetworkBodies()); got != 0 {
		t.Fatalf("GetNetworkBodyCount() after memory eviction = %d, want 0", got)
	}
	if got := c2.buffers.calcNBMemory(); got != 0 {
		t.Fatalf("GetNetworkBodiesBufferMemory() after eviction = %d, want 0", got)
	}
}

func TestCoverageBoost_NetworkWaterfallGetters(t *testing.T) {
	c := newCoverageCapture(t)

	empty := c.NetworkWaterfall().Entries()
	if len(empty) != 0 {
		t.Fatalf("GetNetworkWaterfallEntries() initial len = %d, want 0", len(empty))
	}

	c.networkWaterfall = newNetworkWaterfallStore(1)

	c.NetworkWaterfall().Add([]types.NetworkWaterfallEntry{
		{Name: "https://one.example"},
		{Name: "https://two.example"},
	}, "https://page.example")

	if got := len(c.NetworkWaterfall().Entries()); got != 1 {
		t.Fatalf("GetNetworkWaterfallCount() = %d, want 1", got)
	}
	entries := c.NetworkWaterfall().Entries()
	if len(entries) != 1 {
		t.Fatalf("GetNetworkWaterfallEntries() len = %d, want 1", len(entries))
	}
	if entries[0].PageURL != "https://page.example" {
		t.Fatalf("PageURL = %q, want page URL tag", entries[0].PageURL)
	}
	if entries[0].Timestamp.IsZero() {
		t.Fatal("Timestamp should be set on added waterfall entry")
	}
}

func TestCoverageBoost_ResultHandlersAndPendingQueries(t *testing.T) {
	c := newCoverageCapture(t)

	queryID, _ := c.CreatePendingQueryWithClient(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	}, "client-1")
	if queryID == "" {
		t.Fatal("CreatePendingQueryWithClient returned empty id")
	}

	pending := c.GetPendingQueriesForClient("client-1")
	if len(pending) != 1 {
		t.Fatalf("pending count = %d, want 1", len(pending))
	}

	// Unified /query-result endpoint
	rr := httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodGet, "/query-result", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET query-result status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
	rr = httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader("{bad")))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON query-result status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	rr = httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(`{"id":"q-dom","result":{"ok":true},"client_id":"client-1"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("valid query-result status = %d, want %d", rr.Code, http.StatusOK)
	}
	if _, ok := c.TakeQueryResultForClient("q-dom", "client-1"); !ok {
		t.Fatal("expected q-dom result to be stored for client-1")
	}

	rr = httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(`{"id":"q-a11y","result":{"score":0.9}}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("valid query-result (a11y) status = %d, want %d", rr.Code, http.StatusOK)
	}
	if _, ok := c.TakeQueryResult("q-a11y"); !ok {
		t.Fatal("expected q-a11y result to be stored")
	}

	c.RegisterCommand("corr-1", "q-exec", time.Minute)
	rr = httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(`{"id":"q-exec","correlation_id":"corr-1","status":"complete","result":{"ok":true},"client_id":"client-2"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("valid query-result (execute) status = %d, want %d", rr.Code, http.StatusOK)
	}
	if _, ok := c.TakeQueryResultForClient("q-exec", "client-2"); !ok {
		t.Fatal("expected q-exec result to be stored for client-2")
	}
	if cmd, ok := c.GetCommandResult("corr-1"); !ok || cmd.Status != "complete" {
		t.Fatalf("command result = %+v, ok=%v, want completed command", cmd, ok)
	}

	rr = httptest.NewRecorder()
	c.HandleQueryResult(rr, httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(`{"id":"q-highlight","result":{"found":true}}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("valid query-result (highlight) status = %d, want %d", rr.Code, http.StatusOK)
	}
	if _, ok := c.TakeQueryResult("q-highlight"); !ok {
		t.Fatal("expected q-highlight result to be stored")
	}
}

func TestCoverageBoost_RecordingStorageHandlerAndDelegations(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	c := newCoverageCapture(t)

	rr := httptest.NewRecorder()
	c.HandleRecordingStorage(rr, httptest.NewRequest(http.MethodGet, "/recording/storage", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET recording storage status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = httptest.NewRecorder()
	c.HandleRecordingStorage(rr, httptest.NewRequest(http.MethodDelete, "/recording/storage", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("DELETE missing recording_id status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	rr = httptest.NewRecorder()
	c.HandleRecordingStorage(rr, httptest.NewRequest(http.MethodDelete, "/recording/storage?recording_id=missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("DELETE missing recording status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	recordingID, err := c.StartRecording("coverage", "https://example.test", true)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	if err := c.AddRecordingAction(recording.RecordingAction{Type: "click", Selector: "#btn"}); err != nil {
		t.Fatalf("AddRecordingAction() error = %v", err)
	}
	if _, _, err := c.StopRecording(recordingID); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}

	rr = httptest.NewRecorder()
	c.HandleRecordingStorage(rr, httptest.NewRequest(http.MethodPost, "/recording/storage", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("POST recalculate storage status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = httptest.NewRecorder()
	deleteURL := "/recording/storage?recording_id=" + url.QueryEscape(recordingID)
	c.HandleRecordingStorage(rr, httptest.NewRequest(http.MethodDelete, deleteURL, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE existing recording status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	c.HandleRecordingStorage(rr, httptest.NewRequest(http.MethodPut, "/recording/storage", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT recording storage status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}

	if _, err := c.StartPlayback("missing-recording"); err == nil {
		t.Fatal("StartPlayback(missing) expected error")
	}
	if _, err := c.ExecutePlayback("missing-recording"); err == nil {
		t.Fatal("ExecutePlayback(missing) expected error")
	}
	fragile := c.DetectFragileSelectors([]*playback.Session{
		{Results: []playback.Result{{ActionType: "click", SelectorUsed: "css", Status: "error"}}},
		{Results: []playback.Result{{ActionType: "click", SelectorUsed: "css", Status: "error"}}},
	})
	if !fragile["css:css"] {
		t.Fatalf("DetectFragileSelectors() = %+v, want css:css fragile", fragile)
	}
	statusMap := c.GetPlaybackStatus(&playback.Session{
		StartedAt:        time.Now().Add(-2 * time.Second),
		ActionsExecuted:  0,
		ActionsFailed:    1,
		SelectorFailures: map[string]int{"css": 1},
	})
	if got, _ := statusMap["status"].(string); got != "failed" {
		t.Fatalf("GetPlaybackStatus status = %q, want failed", got)
	}
	if _, err := c.DiffRecordings("orig", "replay"); err == nil {
		t.Fatal("DiffRecordings(orig,replay) expected error for missing recordings")
	}
	cats := c.CategorizeActionTypes(&recording.Recording{
		Actions: []recording.RecordingAction{{Type: "click"}, {Type: "type"}, {Type: "click"}},
	})
	if cats["click"] != 2 || cats["type"] != 1 {
		t.Fatalf("CategorizeActionTypes() = %+v, want click=2,type=1", cats)
	}
	if _, err := c.GetStorageInfo(); err != nil {
		t.Fatalf("GetStorageInfo() error = %v", err)
	}
	if err := c.RecalculateStorageUsed(); err != nil {
		t.Fatalf("RecalculateStorageUsed() error = %v", err)
	}
}
