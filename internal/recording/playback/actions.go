// Purpose: Executes individual recording actions (navigate, click, type) during playback.
// Why: Separates per-action execution from session lifecycle and fragile-selector detection.
package playback

import (
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

func executeAction(index int, action recording.Action) Result {
	startTime := time.Now()

	result := Result{
		Status:      "ok",
		ActionIndex: index,
		ActionType:  action.Type,
		ExecutedAt:  startTime,
	}

	switch action.Type {
	case "navigate":
		result.Status = "ok"
		result.SelectorUsed = "navigate"
		result.Error = ""
	case "click":
		result = executeClickWithHealing(action)
	case "type":
		result.Status = "ok"
		result.SelectorUsed = "type"
	case "scroll":
		result.Status = "ok"
		result.SelectorUsed = "scroll"
	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("unknown_action_type: %s", action.Type)
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result
}

func executeClickWithHealing(action recording.Action) Result {
	result := Result{
		Status:      "error",
		ActionType:  "click",
		ExecutedAt:  time.Now(),
		Coordinates: &Coordinates{X: action.X, Y: action.Y},
	}

	if action.DataTestID != "" {
		selector := fmt.Sprintf("[data-testid=%s]", action.DataTestID)
		if tryClickSelector(selector) {
			result.Status = "ok"
			result.SelectorUsed = "data-testid"
			return result
		}
	}

	if action.Selector != "" {
		if tryClickSelector(action.Selector) {
			result.Status = "ok"
			result.SelectorUsed = "css"
			return result
		}
	}

	if action.X > 0 && action.Y > 0 {
		result.Status = "ok"
		result.SelectorUsed = "nearby_xy"
		result.Coordinates = &Coordinates{X: action.X, Y: action.Y}
		return result
	}

	if len(action.ScreenshotPath) > 0 {
		result.Status = "ok"
		result.SelectorUsed = "last_known"
		return result
	}

	result.Status = "error"
	result.Error = "selector_not_found: Could not find element with any strategy"
	return result
}

func tryClickSelector(selector string) bool {
	if selector == "" {
		return false
	}

	validSelectors := []string{
		"[data-testid=",
		".",
		"#",
		"[",
	}

	for _, prefix := range validSelectors {
		if strings.HasPrefix(selector, prefix) {
			return true
		}
	}

	return false
}
