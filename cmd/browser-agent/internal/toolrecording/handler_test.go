// handler_test.go — Characterization tests for the recording MCP handler boundary.
// Docs: docs/features/feature/flow-recording/index.md

package toolrecording

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	core "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type fakeCapture struct {
	requestedLimit int
	startName      string
	startURL       string
	startSensitive bool
	recordings     []recording.Recording
	lookupErr      error
	stopErr        error
	listErr        error
	recording      *recording.Recording

	activeRecordingID string
	startErr          error
}

// responseText returns the first content block of an MCP response.
func responseText(t *testing.T, response mcp.JSONRPCResponse) string {
	t.Helper()
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("response carried no content")
	}
	return result.Content[0].Text
}

func (f *fakeCapture) StartRecording(name, pageURL string, sensitive bool) (string, error) {
	f.startName = name
	f.startURL = pageURL
	f.startSensitive = sensitive
	if f.startErr != nil {
		return "", f.startErr
	}
	return "rec-123", nil
}

func (f *fakeCapture) StopRecording(string) (int, int64, error) {
	return 3, 125, f.stopErr
}

func (f *fakeCapture) ActiveRecordingID() string { return f.activeRecordingID }

func (f *fakeCapture) ListRecordings(limit int) ([]recording.Recording, error) {
	f.requestedLimit = limit
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
		if name == "list" {
			for _, field := range []string{`"recordings"`, `"count"`, `"limit"`} {
				if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, field) {
					t.Fatalf("recordings response missing %s: %s", field, response.Result)
				}
			}
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

// An already-running recording is an expected condition with an obvious remedy.
// It used to return internal_error with "Check storage quota and try again", so a
// caller acting on the code and the playbook hunted for disk space while the real
// fix went unmentioned — and the id it needed appeared nowhere else.
func TestEventRecordingStartClassifiesAlreadyRecording(t *testing.T) {
	deps := &fakeCapture{startErr: fmt.Errorf("%w: A recording is already active (id: rec-1)", recording.ErrAlreadyRecording)}
	handler := NewHandler(deps, nil)

	resp := handler.EventRecordingStart(mcp.JSONRPCRequest{}, json.RawMessage(`{"name":"x"}`))
	text := responseText(t, resp)

	if !strings.Contains(text, `"error_code":"already_recording"`) {
		t.Fatalf("start must classify the conflict, got: %s", text)
	}
	if strings.Contains(text, "Check storage quota") {
		t.Fatalf("start must not blame storage for an active recording, got: %s", text)
	}
	if !strings.Contains(text, "event_recording_stop") {
		t.Fatalf("the playbook must name the remedy, got: %s", text)
	}
}

// Omitting recording_id stops whatever is active. Requiring it made a lost id
// unrecoverable: the active recording is not listed, and start refuses while one runs.
func TestEventRecordingStopWithoutIDIsAccepted(t *testing.T) {
	handler := NewHandler(&fakeCapture{}, nil)

	resp := handler.EventRecordingStop(mcp.JSONRPCRequest{}, json.RawMessage(`{}`))
	text := responseText(t, resp)

	if strings.Contains(text, "missing_param") {
		t.Fatalf("stop must not demand a recording_id, got: %s", text)
	}
	if !strings.Contains(text, "Recording stopped") {
		t.Fatalf("stop without an id must close the active recording, got: %s", text)
	}
}

// The running recording must be visible to a caller that no longer holds its id.
func TestRecordingsListingExposesTheActiveRecording(t *testing.T) {
	handler := NewHandler(&fakeCapture{activeRecordingID: "rec-live"}, nil)

	text := responseText(t, handler.Recordings(mcp.JSONRPCRequest{}, json.RawMessage(`{}`)))
	if !strings.Contains(text, `"active_recording_id":"rec-live"`) {
		t.Fatalf("recordings listing must expose the active recording, got: %s", text)
	}
}

// observe(recordings) applied a default of 10 but no ceiling, so limit=100000
// built a 2,900,631-byte response from 4761 recordings. The response clamp then
// cut it back to the size limit — and because the recordings array is the last
// key, what survived was the counts with no recordings at all. The caller asked
// for more data and received none.
//
// The observe schema has documented "max 1000" all along; core.MaxObserveLimit
// encodes it and several observe modes apply it. This one did not.
func TestRecordingsCapsAnOverlargeLimit(t *testing.T) {
	capture := &fakeCapture{}
	NewHandler(capture, func(types.LogEntry) {}).
		Recordings(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, json.RawMessage(`{"limit":100000}`))

	if capture.requestedLimit > core.MaxObserveLimit {
		t.Fatalf("store was asked for %d recordings, want it capped at %d", capture.requestedLimit, core.MaxObserveLimit)
	}
}

func TestRecordingsKeepsTheDefaultWhenNoLimitGiven(t *testing.T) {
	capture := &fakeCapture{}
	NewHandler(capture, func(types.LogEntry) {}).
		Recordings(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, nil)
	if capture.requestedLimit != 10 {
		t.Fatalf("default limit = %d, want 10", capture.requestedLimit)
	}
}

// observe(recordings) is a LISTING. It answers "which recordings exist" so a
// caller can pick one; observe(recording_actions) answers "what happened in
// this one". The listing embedded every recording's full actions array, so it
// returned the entire corpus of captured interactions to answer a question
// about names — 480,992 bytes for 1000 recordings, enough to trip the response
// backstop, which then dropped the array wholesale.
//
// Capping the count was treating the symptom. A listing entry has no business
// carrying the actions.
func TestRecordingsListingOmitsActions(t *testing.T) {
	capture := &fakeCapture{recordings: []recording.Recording{{
		ID: "demo-1", Name: "demo", CreatedAt: "2026-08-12T00:00:00Z",
		StartURL: "https://example.test/", Duration: 1200, ActionCount: 3,
		Actions: []recording.RecordingAction{{Type: "click"}, {Type: "type"}, {Type: "click"}},
	}}}

	resp := NewHandler(capture, func(types.LogEntry) {}).
		Recordings(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, nil)

	entry := firstRecordingEntry(t, resp)
	if _, present := entry["actions"]; present {
		t.Error("a listing entry must not carry the recording's actions; use observe(recording_actions) for those")
	}
	// The fields that let a caller choose a recording must survive.
	for _, keep := range []string{"id", "name", "created_at", "action_count", "duration_ms", "start_url"} {
		if _, ok := entry[keep]; !ok {
			t.Errorf("listing entry must keep %q so a caller can choose a recording", keep)
		}
	}
	// action_count is how the caller learns the size without paying for it.
	if count, _ := entry["action_count"].(float64); int(count) != 3 {
		t.Errorf("action_count = %v, want 3", entry["action_count"])
	}
}

func firstRecordingEntry(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		t.Fatalf("response = %s, err=%v", resp.Result, err)
	}
	text := result.Content[0].Text
	start := strings.Index(text, "{")
	if start < 0 {
		t.Fatalf("no JSON body in %.120q", text)
	}
	var payload struct {
		Recordings []map[string]any `json:"recordings"`
	}
	if err := json.Unmarshal([]byte(text[start:]), &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if len(payload.Recordings) == 0 {
		t.Fatal("expected one recording in the listing")
	}
	return payload.Recordings[0]
}
