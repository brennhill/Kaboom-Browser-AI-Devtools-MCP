// recorder.go — Records AI-driven browser actions on the canonical telemetry timeline.
// Why: All entry points must produce identical reproduction metadata and ownership tags.

package actionlog

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Recorder owns normalization of AI actions before they enter telemetry.
type Recorder struct {
	telemetry *telemetrystore.Store
	now       func() time.Time
}

// New constructs an AI action recorder for the canonical telemetry store.
func New(telemetry *telemetrystore.Store) *Recorder {
	return &Recorder{telemetry: telemetry, now: time.Now}
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
}

// RecordEnhanced normalizes and stores a fully populated AI-owned action.
func (r *Recorder) RecordEnhanced(action types.EnhancedAction) {
	action.Timestamp = r.now().UnixMilli()
	action.Source = "ai"
	r.telemetry.AddEnhancedActions([]types.EnhancedAction{action})
}

// RecordDOMPrimitive converts an interact primitive into reproduction metadata.
func (r *Recorder) RecordDOMPrimitive(action, selector, text, value string) {
	reproType, ok := act.DOMActionToReproType[action]
	if !ok {
		r.Record("dom_"+action, "", map[string]any{"selector": selector})
		return
	}
	enhanced := types.EnhancedAction{
		Type: reproType, Selectors: act.ParseSelectorForReproduction(selector),
	}
	switch action {
	case "type":
		enhanced.Value = text
	case "key_press":
		enhanced.Key = text
	case "select":
		enhanced.SelectedValue = value
	}
	r.RecordEnhanced(enhanced)
}
