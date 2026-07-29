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
	stopErr        error
	listErr        error
	recording      *recording.Recording
}

func (f *fakeCapture) StartRecording(name, pageURL string, sensitive bool) (string, error) {
	f.startName = name
	f.startURL = pageURL
	f.startSensitive = sensitive
	return "rec-123", nil
}

func (f *fakeCapture) StopRecording(string) (int, int64, error) {
	return 3, 125, f.stopErr
}

func (f *fakeCapture) ListRecordings(int) ([]recording.Recording, error) {
	return f.recordings, f.listErr
}

func (f *fakeCapture) GetRecording(string) (*recording.Recording, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if f.recording != nil {
		return f.recording, nil
	}
	return &recording.Recording{}, nil
}

func TestRecordingLifecycleAndLookupSuccesses(t *testing.T) {
	rec := recording.Recording{Name: "checkout", StartURL: "https://example.test", ActionCount: 1}
	deps := &fakeCapture{recordings: []recording.Recording{rec}, recording: &rec}
	handler := NewHandler(deps, nil)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for name, response := range map[string]mcp.JSONRPCResponse{
		"stop":       handler.EventRecordingStop(req, json.RawMessage(`{"recording_id":"rec-1"}`)),
		"list":       handler.Recordings(req, json.RawMessage(`{}`)),
		"actions":    handler.RecordingActions(req, json.RawMessage(`{"recording_id":"rec-1"}`)),
		"list limit": handler.Recordings(req, json.RawMessage(`{"limit":2}`)),
	} {
		var result mcp.MCPToolResult
		if err := json.Unmarshal(response.Result, &result); err != nil || result.IsError {
			t.Fatalf("%s response = %s, %v", name, response.Result, err)
		}
	}
}

func TestRecordingLifecycleStorageFailures(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for name, response := range map[string]mcp.JSONRPCResponse{
		"stop":    NewHandler(&fakeCapture{stopErr: errors.New("stop failed")}, nil).EventRecordingStop(req, json.RawMessage(`{"recording_id":"rec-1"}`)),
		"list":    NewHandler(&fakeCapture{listErr: errors.New("list failed")}, nil).Recordings(req, nil),
		"actions": NewHandler(&fakeCapture{lookupErr: errors.New("load failed")}, nil).RecordingActions(req, json.RawMessage(`{"recording_id":"rec-1"}`)),
		"playback": NewHandler(&fakeCapture{lookupErr: errors.New("load failed")}, nil).
			Playback(req, json.RawMessage(`{"recording_id":"rec-1"}`)),
		"results": NewHandler(&fakeCapture{}, nil).
			PlaybackResults(req, json.RawMessage(`{"recording_id":"rec-1"}`)),
	} {
		var result mcp.MCPToolResult
		if err := json.Unmarshal(response.Result, &result); err != nil || !result.IsError {
			t.Fatalf("%s response = %s, %v", name, response.Result, err)
		}
	}
}

func TestDiffParamsRequireBothRecordingIDs(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for _, args := range []json.RawMessage{
		nil,
		json.RawMessage(`{"original_id":"before"}`),
		json.RawMessage(`bad`),
	} {
		if _, response := parseDiffParams(req, args); response == nil {
			t.Fatalf("args %s unexpectedly accepted", args)
		}
	}
}

func TestPlaybackAndResultsRoundTrip(t *testing.T) {
	rec := &recording.Recording{
		ID: "flow",
		Actions: []recording.RecordingAction{
			{Type: "navigate"},
			{Type: "click", Selector: "#save", X: 10, Y: 20},
			{Type: "type", Text: "hello"},
		},
	}
	handler := NewHandler(&fakeCapture{recording: rec}, nil)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	playbackResponse := handler.Playback(req, json.RawMessage(`{"recording_id":"flow"}`))
	var playbackResult mcp.MCPToolResult
	if err := json.Unmarshal(playbackResponse.Result, &playbackResult); err != nil || playbackResult.IsError {
		t.Fatalf("playback = %s, %v", playbackResponse.Result, err)
	}
	resultsResponse := handler.PlaybackResults(req, json.RawMessage(`{"recording_id":"flow"}`))
	var results mcp.MCPToolResult
	if err := json.Unmarshal(resultsResponse.Result, &results); err != nil || results.IsError {
		t.Fatalf("results = %s, %v", resultsResponse.Result, err)
	}
	if !strings.Contains(string(resultsResponse.Result), `actions_total`) ||
		!strings.Contains(string(resultsResponse.Result), `coordinates`) {
		t.Fatalf("results missing action details: %s", resultsResponse.Result)
	}
}

func (f *fakeCapture) LookupRecording(string) (*recording.Recording, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if f.recording != nil {
		return f.recording, nil
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
