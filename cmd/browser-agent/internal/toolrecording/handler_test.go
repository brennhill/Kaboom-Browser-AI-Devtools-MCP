// handler_test.go — Characterization tests for the recording MCP handler boundary.
// Docs: docs/features/feature/flow-recording/index.md

package toolrecording

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type fakeCapture struct {
	startName      string
	startURL       string
	startSensitive bool
	recordings     []recording.Recording
	lookupErr      error
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
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return &recording.Recording{}, nil
}

func (f *fakeCapture) LookupRecording(string) (*recording.Recording, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return &recording.Recording{}, nil
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

func TestLogDiffHandlersPreserveOperationSpecificFailures(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	args := json.RawMessage(`{"original_id":"before","replay_id":"after"}`)

	tests := []struct {
		name string
		run  func(*Handler, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
		want string
	}{
		{name: "summary", run: (*Handler).LogDiff, want: "Failed to diff recordings"},
		{name: "report", run: (*Handler).LogDiffReport, want: "Failed to generate report"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NewHandler(&fakeCapture{lookupErr: errors.New("unavailable")}, nil)
			resp := tt.run(handler, req, args)
			if !strings.Contains(string(resp.Result), tt.want) {
				t.Fatalf("response %s does not contain %q", string(resp.Result), tt.want)
			}
		})
	}
}

func TestRecordingIDHandlersRejectInvalidJSON(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 4}
	tests := []struct {
		name string
		run  func(*Handler, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	}{
		{name: "stop", run: (*Handler).EventRecordingStop},
		{name: "actions", run: (*Handler).RecordingActions},
		{name: "playback", run: (*Handler).Playback},
		{name: "playback results", run: (*Handler).PlaybackResults},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := tt.run(NewHandler(&fakeCapture{}, nil), req, json.RawMessage(`bad`))
			if !strings.Contains(string(resp.Result), mcp.ErrInvalidJSON) {
				t.Fatalf("expected %s, got %s", mcp.ErrInvalidJSON, string(resp.Result))
			}
		})
	}
}
