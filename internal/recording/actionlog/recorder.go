// recorder.go — Records AI-driven browser actions on the canonical telemetry timeline.
// Why: All entry points must produce identical reproduction metadata and ownership tags.

package actionlog

import (
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/store/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
)

// Recorder owns normalization of AI actions before they enter telemetry.
type Recorder struct {
	telemetry *telemetrystore.Store
	now       func() time.Time
	// flow receives every action while a flow recording is active (kaboom-fpup).
	flow        FlowSink
	onFlowError func(error)
}

// FlowSink is the active flow recording: it keeps an action only while recording.
type FlowSink interface {
	RecordIfActive(action recording.RecordingAction) (bool, error)
}

// SetFlowSink makes every recorded action also feed the active flow recording.
func (r *Recorder) SetFlowSink(sink FlowSink) { r.flow = sink }

// New constructs an AI action recorder for the canonical telemetry store.
func New(telemetry *telemetrystore.Store) *Recorder {
	return &Recorder{telemetry: telemetry, now: time.Now, onFlowError: logFlowError}
}

// Record creates and stores an AI-owned action.
func (r *Recorder) Record(actionType, url string, details map[string]any) {
	action := types.EnhancedAction{
		Type: actionType, Timestamp: r.now().UnixMilli(), URL: url, Source: "ai",
	}
	if len(details) > 0 {
		action.Selectors = details
	}
	r.telemetry.AddEnhancedActions([]types.EnhancedAction{action})
	selector, _ := details["selector"].(string)
	r.feedFlow(action, selector)
}

// RecordEnhanced normalizes and stores a fully populated AI-owned action.
func (r *Recorder) RecordEnhanced(action types.EnhancedAction) {
	r.recordEnhanced(action, "")
}

func (r *Recorder) recordEnhanced(action types.EnhancedAction, selector string) {
	action.Timestamp = r.now().UnixMilli()
	action.Source = "ai"
	r.telemetry.AddEnhancedActions([]types.EnhancedAction{action})
	r.feedFlow(action, selector)
}

// RecordDOMPrimitive converts an interact primitive into reproduction metadata.
func (r *Recorder) RecordDOMPrimitive(action, selector, text, value string) {
	reproType, ok := reproTypeForDOMAction(action)
	if !ok {
		r.Record("dom_"+action, "", map[string]any{"selector": selector})
		return
	}
	enhanced := types.EnhancedAction{
		Type: reproType, Selectors: parseSelectorForReproduction(selector),
	}
	switch action {
	case "type":
		enhanced.Value = text
	case "key_press":
		enhanced.Key = text
	case "select":
		enhanced.SelectedValue = value
	}
	r.recordEnhanced(enhanced, selector)
}

// feedFlow mirrors a recorded action into the active flow recording.
func (r *Recorder) feedFlow(action types.EnhancedAction, selector string) {
	if r.flow == nil {
		return
	}
	text := action.Value
	if text == "" {
		text = action.Key
	}
	if text == "" {
		text = action.SelectedValue
	}
	flowAction := recording.RecordingAction{
		Type: action.Type, TimestampMs: action.Timestamp, URL: action.URL, Selector: selector, Text: text,
	}
	if _, err := r.flow.RecordIfActive(flowAction); err != nil && r.onFlowError != nil {
		r.onFlowError(err)
	}
}

func logFlowError(err error) {
	fmt.Fprintf(os.Stderr, "[Kaboom] flow recording dropped an action: %v\n", err)
}
