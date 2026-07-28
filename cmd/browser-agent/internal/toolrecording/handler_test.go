// handler_test.go — Characterization tests for the recording MCP handler boundary.
// Docs: docs/features/feature/flow-recording/index.md

package toolrecording

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type fakeCapture struct {
	startName      string
	startURL       string
	startSensitive bool
	recordings     []recording.Recording
}

func (f *fakeCapture) StartRecording(name, pageURL string, sensitive bool) (string, error) {
	f.startName = name
	f.startURL = pageURL
	f.startSensitive = sensitive
	return "rec-123", nil
}

func (f *fakeCapture) StopRecording(string) (int, int64, error) {
	return 3, 125, nil
}

func (f *fakeCapture) ListRecordings(int) ([]recording.Recording, error) {
	return f.recordings, nil
}

func (f *fakeCapture) GetRecording(string) (*recording.Recording, error) {
	return &recording.Recording{}, nil
}

func (f *fakeCapture) ExecutePlayback(string) (*playback.Session, error) {
	return &playback.Session{}, nil
}

func (f *fakeCapture) DiffRecordings(string, string) (*logdiff.Result, error) {
	return &logdiff.Result{}, nil
}

func TestHandlerEventRecordingStartDefaultsURLAndLogs(t *testing.T) {
	deps := &fakeCapture{}
	var logs []types.LogEntry
	handler := NewHandler(deps, func(entry types.LogEntry) {
		logs = append(logs, entry)
	})

	resp := handler.EventRecordingStart(
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1},
		json.RawMessage(`{"name":"checkout","sensitive_data_enabled":true}`),
	)

	if resp.Error != nil {
		t.Fatalf("EventRecordingStart returned JSON-RPC error: %v", resp.Error)
	}
	if deps.startName != "checkout" || deps.startURL != "about:blank" || !deps.startSensitive {
		t.Fatalf("unexpected start call: name=%q url=%q sensitive=%v", deps.startName, deps.startURL, deps.startSensitive)
	}
	if len(logs) != 1 || logs[0]["recording_id"] != "rec-123" {
		t.Fatalf("expected one recording log, got %#v", logs)
	}
}

func TestHandlerRecordingActionsRequiresRecordingID(t *testing.T) {
	handler := NewHandler(&fakeCapture{}, nil)
	resp := handler.RecordingActions(
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2},
		json.RawMessage(`{}`),
	)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if !result.IsError || len(result.Content) == 0 {
		t.Fatalf("expected structured missing-parameter error, got %#v", result)
	}
}
