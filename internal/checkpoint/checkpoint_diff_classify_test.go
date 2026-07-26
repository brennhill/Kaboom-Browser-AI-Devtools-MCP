// Purpose: Tests for checkpoint error classification and severity assignment.

// ai_checkpoint_classify_test.go — Tests for WS classification, log extraction, action diffs, and JSON serialization.
package checkpoint

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/server"
	gasTypes "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// capWSDiff: capping all fields
// ============================================

func TestCapWSDiff_CapsAllFields(t *testing.T) {
	t.Parallel()

	diff := &WebSocketDiff{}
	for i := 0; i < maxDiffEntriesPerCat+10; i++ {
		diff.Disconnections = append(diff.Disconnections, WSDisconnection{URL: fmt.Sprintf("ws://%d", i)})
		diff.Connections = append(diff.Connections, WSConn{URL: fmt.Sprintf("ws://%d", i)})
		diff.Errors = append(diff.Errors, WSError{URL: fmt.Sprintf("ws://%d", i)})
	}

	capWSDiff(diff)

	if len(diff.Disconnections) != maxDiffEntriesPerCat {
		t.Errorf("Disconnections len = %d, want %d", len(diff.Disconnections), maxDiffEntriesPerCat)
	}
	if len(diff.Connections) != maxDiffEntriesPerCat {
		t.Errorf("Connections len = %d, want %d", len(diff.Connections), maxDiffEntriesPerCat)
	}
	if len(diff.Errors) != maxDiffEntriesPerCat {
		t.Errorf("Errors len = %d, want %d", len(diff.Errors), maxDiffEntriesPerCat)
	}
}

// ============================================
// capNetworkDiff: below cap (no-op)
// ============================================

func TestCapNetworkDiff_BelowCapNoOp(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{
		Failures:     []NetworkFailure{{Path: "/a"}},
		NewEndpoints: []string{"/b"},
		Degraded:     []NetworkDegraded{{Path: "/c"}},
	}

	capNetworkDiff(diff)

	if len(diff.Failures) != 1 || len(diff.NewEndpoints) != 1 || len(diff.Degraded) != 1 {
		t.Error("cap should not modify diffs below the limit")
	}
}

// ============================================
// classifyWSEvent: all event types
// ============================================

func TestClassifyWSEvent_AllTypes(t *testing.T) {
	t.Parallel()

	// open event
	diff := &WebSocketDiff{}
	openEvt := capture.WebSocketEvent{Event: "open", URL: "ws://a", ID: "id1"}
	classifyWSEvent(diff, &openEvt, "all")
	if len(diff.Connections) != 1 || diff.Connections[0].URL != "ws://a" || diff.Connections[0].ID != "id1" {
		t.Errorf("open event not classified correctly: %+v", diff.Connections)
	}

	// close event
	diff = &WebSocketDiff{}
	closeEvt := capture.WebSocketEvent{Event: "close", URL: "ws://b", CloseCode: 1006, CloseReason: "abnormal"}
	classifyWSEvent(diff, &closeEvt, "all")
	if len(diff.Disconnections) != 1 {
		t.Fatalf("close event not classified correctly: %+v", diff)
	}
	if diff.Disconnections[0].CloseCode != 1006 || diff.Disconnections[0].CloseReason != "abnormal" {
		t.Errorf("close event fields wrong: %+v", diff.Disconnections[0])
	}

	// close event with errors_only severity should be skipped
	diff = &WebSocketDiff{}
	classifyWSEvent(diff, &closeEvt, "errors_only")
	if len(diff.Disconnections) != 0 {
		t.Error("close event should be skipped in errors_only mode")
	}

	// error event
	diff = &WebSocketDiff{}
	errEvt := capture.WebSocketEvent{Event: "error", URL: "ws://c", Data: "conn reset"}
	classifyWSEvent(diff, &errEvt, "all")
	if len(diff.Errors) != 1 || diff.Errors[0].Message != "conn reset" {
		t.Errorf("error event not classified correctly: %+v", diff.Errors)
	}

	// unknown event type (no classification)
	diff = &WebSocketDiff{}
	unknownEvt := capture.WebSocketEvent{Event: "message", URL: "ws://d"}
	classifyWSEvent(diff, &unknownEvt, "all")
	if len(diff.Connections) != 0 && len(diff.Disconnections) != 0 && len(diff.Errors) != 0 {
		t.Error("unknown event type should not be classified")
	}
}

// ============================================
// extractLogMessage: msg vs message field
// ============================================

func TestExtractLogMessage_Fields(t *testing.T) {
	t.Parallel()

	// "msg" field preferred
	entry := gasTypes.LogEntry{"msg": "primary", "message": "secondary"}
	if got := extractLogMessage(entry); got != "primary" {
		t.Errorf("extractLogMessage with both fields = %q, want 'primary'", got)
	}

	// Fallback to "message"
	entry = gasTypes.LogEntry{"message": "fallback"}
	if got := extractLogMessage(entry); got != "fallback" {
		t.Errorf("extractLogMessage fallback = %q, want 'fallback'", got)
	}

	// Neither field present
	entry = gasTypes.LogEntry{"level": "info"}
	if got := extractLogMessage(entry); got != "" {
		t.Errorf("extractLogMessage missing = %q, want empty", got)
	}
}

// ============================================
// buildConsoleEntries: cap at maxDiffEntriesPerCat
// ============================================

func TestBuildConsoleEntries_Cap(t *testing.T) {
	t.Parallel()

	m := make(map[string]*fingerprintEntry)
	var order []string
	for i := 0; i < maxDiffEntriesPerCat+10; i++ {
		key := fmt.Sprintf("msg_%d", i)
		m[key] = &fingerprintEntry{message: key, source: "src", count: 1}
		order = append(order, key)
	}

	entries := buildConsoleEntries(m, order)
	if len(entries) != maxDiffEntriesPerCat {
		t.Errorf("buildConsoleEntries len = %d, want %d", len(entries), maxDiffEntriesPerCat)
	}
}

// ============================================
// computeConsoleDiff: negative recentSlice (evicted buffer)
// ============================================

func TestComputeConsoleDiff_NegativeRecentSlice(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{
			Entries:    []gasTypes.LogEntry{},
			TotalAdded: 0,
		},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	// Checkpoint claims more entries than buffer has (buffer eviction)
	cp := &Checkpoint{LogTotal: 100}
	diff := cm.computeConsoleDiff(cp, "all")
	if diff.TotalNew != 0 {
		t.Errorf("TotalNew = %d, want 0 for evicted buffer", diff.TotalNew)
	}
}

// ============================================
// computeNetworkDiff: negative recentSlice
// ============================================

func TestComputeNetworkDiff_NegativeRecentSlice(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cap := capture.NewCapture()
	cm := NewCheckpointManager(fake, cap)

	cp := &Checkpoint{NetworkTotal: 100, KnownEndpoints: map[string]endpointState{}}
	diff := cm.computeNetworkDiff(cp)
	if diff.TotalNew != 0 {
		t.Errorf("TotalNew = %d, want 0 for evicted buffer", diff.TotalNew)
	}
}

// ============================================
// computeWebSocketDiff: negative recentSlice
// ============================================

func TestComputeWebSocketDiff_NegativeRecentSlice(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cap := capture.NewCapture()
	cm := NewCheckpointManager(fake, cap)

	cp := &Checkpoint{WSTotal: 100}
	diff := cm.computeWebSocketDiff(cp, "all")
	if diff.TotalNew != 0 {
		t.Errorf("TotalNew = %d, want 0 for evicted buffer", diff.TotalNew)
	}
}

// ============================================
// computeActionsDiff: negative recentSlice
// ============================================

func TestComputeActionsDiff_NegativeRecentSlice(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cap := capture.NewCapture()
	cm := NewCheckpointManager(fake, cap)

	cp := &Checkpoint{ActionTotal: 100}
	diff := cm.computeActionsDiff(cp)
	if diff.TotalNew != 0 {
		t.Errorf("TotalNew = %d, want 0 for evicted buffer", diff.TotalNew)
	}
}

// ============================================
// computeActionsDiff: cap at maxDiffEntriesPerCat
// ============================================

func TestComputeActionsDiff_Cap(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cap := capture.NewCapture()

	actions := make([]capture.EnhancedAction, maxDiffEntriesPerCat+10)
	for i := range actions {
		actions[i] = capture.EnhancedAction{
			Type:      "click",
			URL:       fmt.Sprintf("http://localhost/%d", i),
			Timestamp: int64(i),
		}
	}
	cap.AddEnhancedActions(actions)

	cm := NewCheckpointManager(fake, cap)
	cp := &Checkpoint{ActionTotal: 0}
	diff := cm.computeActionsDiff(cp)

	if diff.TotalNew != maxDiffEntriesPerCat+10 {
		t.Errorf("TotalNew = %d, want %d", diff.TotalNew, maxDiffEntriesPerCat+10)
	}
	if len(diff.Actions) != maxDiffEntriesPerCat {
		t.Errorf("Actions len = %d, want %d (capped)", len(diff.Actions), maxDiffEntriesPerCat)
	}
}

// ============================================
// shouldInclude: various inputs
// ============================================

func TestShouldInclude(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}

	// nil include = include all
	if !cm.shouldInclude(nil, "console") {
		t.Error("nil include should include all categories")
	}
	if !cm.shouldInclude(nil, "network") {
		t.Error("nil include should include all categories")
	}

	// Empty include = include all
	if !cm.shouldInclude([]string{}, "console") {
		t.Error("empty include should include all categories")
	}

	// Specific include
	if !cm.shouldInclude([]string{"console", "network"}, "console") {
		t.Error("console should be included")
	}
	if cm.shouldInclude([]string{"console", "network"}, "websocket") {
		t.Error("websocket should not be included when not in list")
	}
}

// ============================================
// DiffResponse JSON: snake_case fields
// ============================================

func TestDiffResponse_JSONSnakeCase(t *testing.T) {
	t.Parallel()

	resp := DiffResponse{
		From:       time.Now(),
		To:         time.Now(),
		DurationMs: 100,
		Severity:   "clean",
		Summary:    "test",
		TokenCount: 42,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)
	expectedFields := []string{`"from"`, `"to"`, `"duration_ms"`, `"severity"`, `"summary"`, `"token_count"`}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON missing snake_case field %s, got: %s", field, jsonStr)
		}
	}
}

// ============================================
// classifyLogEntries: all levels
// ============================================

func TestClassifyLogEntries_AllLevels(t *testing.T) {
	t.Parallel()

	entries := []gasTypes.LogEntry{
		{"level": "error", "message": "err1"},
		{"level": "warn", "message": "warn1"},
		{"level": "warning", "message": "warn2"},
		{"level": "info", "message": "info1"},
		{"level": "debug", "message": "debug1"},
	}

	cl := classifyLogEntries(entries, "all")
	if cl.totalNew != 5 {
		t.Errorf("totalNew = %d, want 5", cl.totalNew)
	}
	if len(cl.errorMap) != 1 {
		t.Errorf("errorMap len = %d, want 1", len(cl.errorMap))
	}
	if len(cl.warningMap) != 2 {
		t.Errorf("warningMap len = %d, want 2 (warn + warning)", len(cl.warningMap))
	}

	// errors_only mode: no warnings
	cl2 := classifyLogEntries(entries, "errors_only")
	if len(cl2.warningMap) != 0 {
		t.Errorf("errors_only mode: warningMap len = %d, want 0", len(cl2.warningMap))
	}
}

// ============================================
// classifyFailedRequest: known endpoint already failing
// ============================================

func TestClassifyFailedRequest_KnownEndpointAlreadyFailing(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	known := map[string]endpointState{
		"/api": {Status: 500, Duration: 100}, // already failing
	}

	classifyFailedRequest(diff, "/api", 503, known)

	// Status transition from 500 to 503 should not register as a new failure
	// because previous status was already >= 400
	if len(diff.Failures) != 0 {
		t.Errorf("expected 0 failures for already-failing endpoint, got %d", len(diff.Failures))
	}
	if len(diff.NewEndpoints) != 0 {
		t.Errorf("expected 0 new endpoints for already-known endpoint, got %d", len(diff.NewEndpoints))
	}
}
