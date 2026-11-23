// flow_test.go — Tests that every AI interact action also reaches the active flow recording (kaboom-fpup).
package actionlog

import (
	"errors"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

type fakeFlowSink struct {
	actions []recording.RecordingAction
	err     error
}

func (f *fakeFlowSink) RecordIfActive(action recording.RecordingAction) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.actions = append(f.actions, action)
	return true, nil
}

func TestRecorderFeedsEveryActionKindToFlowRecording(t *testing.T) {
	recorder, _ := newTestRecorder()
	sink := &fakeFlowSink{}
	recorder.SetFlowSink(sink)

	recorder.RecordDOMPrimitive("type", "#email", "a@b.c", "")
	recorder.RecordDOMPrimitive("get_text", "#title", "", "")
	recorder.Record("navigate", "https://example.test/", nil)
	recorder.Record("upload", "", map[string]any{"selector": "#file"})

	if len(sink.actions) != 4 {
		t.Fatalf("flow recording got %d actions, want 4 (every interact action): %+v", len(sink.actions), sink.actions)
	}
	if a := sink.actions[0]; a.Type != "input" || a.Selector != "#email" || a.Text != "a@b.c" || a.TimestampMs != 1234 {
		t.Fatalf("typed action = %+v, want input on #email with text and timestamp", a)
	}
	if a := sink.actions[1]; a.Type != "dom_get_text" || a.Selector != "#title" {
		t.Fatalf("unmapped DOM action = %+v, want dom_get_text on #title", a)
	}
	if a := sink.actions[2]; a.Type != "navigate" || a.URL != "https://example.test/" {
		t.Fatalf("navigate action = %+v", a)
	}
	if a := sink.actions[3]; a.Type != "upload" || a.Selector != "#file" {
		t.Fatalf("upload action = %+v", a)
	}
}

func TestRecorderWithoutFlowSinkStillRecordsTelemetry(t *testing.T) {
	recorder, store := newTestRecorder()
	recorder.Record("navigate", "https://example.test/", nil)
	if got := len(store.Telemetry().Actions().Snapshot().Actions); got != 1 {
		t.Fatalf("telemetry actions = %d, want 1", got)
	}
}

func TestRecorderReportsFlowSinkFailure(t *testing.T) {
	recorder, store := newTestRecorder()
	var reported error
	recorder.SetFlowSink(&fakeFlowSink{err: errors.New("recording_missing: Active recording not found")})
	recorder.onFlowError = func(err error) { reported = err }
	recorder.Record("navigate", "https://example.test/", nil)
	if reported == nil {
		t.Fatal("flow sink failure must be reported, not swallowed")
	}
	if got := len(store.Telemetry().Actions().Snapshot().Actions); got != 1 {
		t.Fatalf("telemetry must still record the action, got %d", got)
	}
}
