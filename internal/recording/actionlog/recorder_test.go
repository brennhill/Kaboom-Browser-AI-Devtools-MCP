// recorder_test.go — Verifies canonical AI action timeline recording.

package actionlog

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func newTestRecorder() (*Recorder, *capture.Capture) {
	store := capture.NewCapture()
	recorder := New(store.Telemetry())
	recorder.now = func() time.Time { return time.UnixMilli(1234) }
	return recorder, store
}

func TestRecorderRecordsCanonicalAIAction(t *testing.T) {
	recorder, store := newTestRecorder()

	recorder.Record("navigate", "https://example.com", map[string]any{"tab_id": 4})

	actions := store.Telemetry().Actions().Snapshot().Actions
	if len(actions) != 1 {
		t.Fatalf("actions = %d, want 1", len(actions))
	}
	got := actions[0]
	if got.Type != "navigate" || got.URL != "https://example.com" || got.Source != "ai" || got.Timestamp != 1234 {
		t.Fatalf("unexpected action: %#v", got)
	}
	if got.Selectors["tab_id"] != 4 {
		t.Fatalf("selectors = %#v, want tab_id=4", got.Selectors)
	}
}

func TestRecorderNormalizesEnhancedActionOwnership(t *testing.T) {
	recorder, store := newTestRecorder()

	recorder.RecordEnhanced(types.EnhancedAction{
		Type: "click", Source: "browser", Timestamp: 99, Value: "kept",
	})

	got := store.Telemetry().Actions().Snapshot().Actions[0]
	if got.Source != "ai" || got.Timestamp != 1234 || got.Value != "kept" {
		t.Fatalf("unexpected enhanced action: %#v", got)
	}
}

func TestRecorderMapsDOMPrimitiveForReproduction(t *testing.T) {
	recorder, store := newTestRecorder()

	recorder.RecordDOMPrimitive("type", "#email", "person@example.com", "")

	got := store.Telemetry().Actions().Snapshot().Actions[0]
	if got.Type != "input" || got.Value != "person@example.com" || got.Source != "ai" {
		t.Fatalf("unexpected DOM action: %#v", got)
	}
	if got.Selectors["id"] != "email" {
		t.Fatalf("selectors = %#v, want normalized ID selector", got.Selectors)
	}
}

func TestRecorderPreservesUnknownDOMPrimitive(t *testing.T) {
	recorder, store := newTestRecorder()

	recorder.RecordDOMPrimitive("custom", "#target", "", "")

	got := store.Telemetry().Actions().Snapshot().Actions[0]
	if got.Type != "dom_custom" || got.Selectors["selector"] != "#target" {
		t.Fatalf("unexpected fallback action: %#v", got)
	}
}
