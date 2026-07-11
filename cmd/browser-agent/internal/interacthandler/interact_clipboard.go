// Purpose: Handles clipboard read and paste actions via navigator.clipboard API for testing copy/paste interactions.
// Why: Enables agents to verify clipboard content without injecting arbitrary JavaScript.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
)

// handleClipboardRead reads text from the clipboard via navigator.clipboard.readText().
func (h *InteractActionHandler) HandleClipboardRead(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	script := `(async () => {
  try {
    const text = await navigator.clipboard.readText();
    return { text };
  } catch (e) {
    return { error: 'clipboard_read_failed', message: e.message };
  }
})()`

	resp := h.queueExecuteScript(req, args, "exec", 0, 0, "main", script, "clipboard_read", "Clipboard read queued")

	// Record AI action only on success (queueExecuteScript handles guards).
	if !isErrorResponse(resp) {
		h.deps.RecordAIAction("clipboard_read", "", nil)
	}

	return resp
}

// handleClipboardWrite writes text to the clipboard via navigator.clipboard.writeText().
func (h *InteractActionHandler) HandleClipboardWrite(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text string `json:"text"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := requireString(req, params.Text, "text", "Add the 'text' parameter with the content to write to clipboard"); blocked {
		return resp
	}

	// JSON-encode the text to safely embed it in the script
	textBytes, _ := json.Marshal(params.Text)

	script := `(async () => {
  try {
    await navigator.clipboard.writeText(` + string(textBytes) + `);
    return { success: true };
  } catch (e) {
    return { error: 'clipboard_write_failed', message: e.message };
  }
})()`

	resp := h.queueExecuteScript(req, args, "exec", 0, 0, "main", script, "clipboard_write", "Clipboard write queued")

	// Record AI action only on success (queueExecuteScript handles guards).
	if !isErrorResponse(resp) {
		h.deps.RecordAIAction("clipboard_write", "", map[string]any{"text_length": len(params.Text)})
	}

	return resp
}
