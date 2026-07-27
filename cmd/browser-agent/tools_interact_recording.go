// Purpose: Records interact and DOM primitive actions in enhanced action history.
// Why: AI action capture and reproduction mapping write the same history contract.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// recordAIAction records an AI-driven action to the enhanced actions buffer.
func (h *ToolHandler) recordAIAction(actionType string, url string, details map[string]any) {
	action := capture.EnhancedAction{
		Type:      actionType,
		Timestamp: time.Now().UnixMilli(),
		URL:       url,
		Source:    "ai",
	}
	if len(details) > 0 {
		action.Selectors = details
	}
	h.capture.AddEnhancedActions([]capture.EnhancedAction{action})
}

// recordAIEnhancedAction records a fully populated AI-driven action.
func (h *ToolHandler) recordAIEnhancedAction(action capture.EnhancedAction) {
	action.Timestamp = time.Now().UnixMilli()
	action.Source = "ai"
	h.capture.AddEnhancedActions([]capture.EnhancedAction{action})
}

// recordDOMPrimitiveAction maps a DOM primitive into a reproduction-compatible action.
func (h *ToolHandler) recordDOMPrimitiveAction(action, selector, text, value string) {
	reproType, ok := act.DOMActionToReproType[action]
	if !ok {
		h.recordAIAction("dom_"+action, "", map[string]any{"selector": selector})
		return
	}

	enhanced := capture.EnhancedAction{
		Type:      reproType,
		Selectors: act.ParseSelectorForReproduction(selector),
	}
	switch action {
	case "type":
		enhanced.Value = text
	case "key_press":
		enhanced.Key = text
	case "select":
		enhanced.SelectedValue = value
	}
	h.recordAIEnhancedAction(enhanced)
}
