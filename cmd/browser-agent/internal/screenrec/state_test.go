// state_test.go — Tests recording lifecycle state resolution.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestExtractRecordingLifecycleStatus(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: "", want: ""},
		{raw: "{bad", want: ""},
		{raw: `{"status":" RECORDING "}`, want: "recording"},
	} {
		if got := extractRecordingLifecycleStatus(json.RawMessage(tc.raw)); got != tc.want {
			t.Fatalf("%q => %q", tc.raw, got)
		}
	}
}

func TestResolveInteractRecordingStateTransitions(t *testing.T) {
	results := map[string]*queries.CommandResult{}
	handler := NewInteractHandler(Deps{GetCommandResult: func(id string) (*queries.CommandResult, bool) {
		result, ok := results[id]
		return result, ok
	}})

	if got := handler.resolveInteractRecordingState(); got.State != recordingStateIdle {
		t.Fatalf("empty state = %+v", got)
	}
	handler.setInteractRecordingStart("start")
	if got := handler.resolveInteractRecordingState(); got.State != recordingStateAwaitingGesture {
		t.Fatalf("missing start result = %+v", got)
	}
	results["start"] = &queries.CommandResult{Status: "pending"}
	if got := handler.resolveInteractRecordingState(); got.State != recordingStateAwaitingGesture {
		t.Fatalf("pending start = %+v", got)
	}
	results["start"] = &queries.CommandResult{Status: "complete", Result: json.RawMessage(`{"status":"recording"}`)}
	if got := handler.resolveInteractRecordingState(); got.State != recordingStateRecording {
		t.Fatalf("complete start = %+v", got)
	}
	handler.setInteractRecordingStopping("stop")
	results["stop"] = &queries.CommandResult{Status: "pending"}
	if got := handler.resolveInteractRecordingState(); got.State != recordingStateStopping {
		t.Fatalf("pending stop = %+v", got)
	}
	results["stop"] = &queries.CommandResult{Status: "complete"}
	if got := handler.resolveInteractRecordingState(); got.State != recordingStateIdle || got.StopCorrelationID != "" {
		t.Fatalf("complete stop = %+v", got)
	}
}

func TestResolveInteractRecordingTerminalAndUnknownResults(t *testing.T) {
	for _, result := range []*queries.CommandResult{
		{Status: "error"},
		{Status: "complete", Result: json.RawMessage(`{"status":"awaiting_gesture"}`)},
		{Status: "complete", Result: json.RawMessage(`{"status":"unexpected"}`)},
	} {
		handler := NewInteractHandler(Deps{GetCommandResult: func(string) (*queries.CommandResult, bool) {
			return result, true
		}})
		handler.setInteractRecordingStart("start")
		got := handler.resolveInteractRecordingState()
		if result.Status == "complete" && extractRecordingLifecycleStatus(result.Result) == recordingStateAwaitingGesture {
			if got.State != recordingStateAwaitingGesture {
				t.Fatalf("awaiting result = %+v", got)
			}
		} else if got.State != recordingStateIdle {
			t.Fatalf("terminal result = %+v", got)
		}
	}
}
