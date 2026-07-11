// Purpose: Updates server-side tracked tab state (tab_id, url, title) after switch_tab completes successfully.
// Why: Keeps the server's active tab reference in sync with the browser so subsequent actions target the correct tab.
// Docs: docs/features/feature/tab-tracking-ux/index.md

package interacthandler

import "encoding/json"

// applySwitchTabTracking extracts tab_id/url/title from a completed switch_tab
// response and updates the server-side tracked tab state.
// Only updates on success (status=complete, success=true, tab_id present).
//
// NOTE: This only runs in synchronous mode (the default). In async mode
// (background=true), server-side tracking is NOT immediately updated.
// The extension-side persistTrackedTab handles async retarget via the
// next /sync heartbeat. See issue #271.
func (h *InteractActionHandler) ApplySwitchTabTracking(correlationID string) {
	cmd, found := h.deps.Capture().GetCommandResult(correlationID)
	if !found || cmd == nil || cmd.Status != "complete" {
		return
	}

	var result map[string]any
	if err := json.Unmarshal(cmd.Result, &result); err != nil {
		return
	}

	success, _ := result["success"].(bool)
	if !success {
		return
	}

	tabIDFloat, _ := result["tab_id"].(float64)
	tabID := int(tabIDFloat)
	if tabID <= 0 {
		return
	}

	tabURL, _ := result["url"].(string)
	tabTitle, _ := result["title"].(string)
	h.deps.Capture().UpdateTrackedTab(tabID, tabURL, tabTitle)
}
