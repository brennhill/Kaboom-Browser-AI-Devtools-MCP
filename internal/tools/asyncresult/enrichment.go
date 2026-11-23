// enrichment.go — Enriches async command payloads with agent-facing context and recovery hints.
// Why: Centralizes response metadata promotion and failure diagnosis in one module.

package asyncresult

import (
	"encoding/json"
	"fmt"
	"strings"
)

// enrichedFieldKeys lists the fields that EnrichCommandResponseData() surfaces
// from the inner extension result to the top-level response. Both the enrichment
// loop and StripEnrichedFieldsFromResult() reference this slice so the two sides
// cannot diverge.
var enrichedFieldKeys = []string{
	"timing", "dom_changes", "dom_summary", "dom_mutations", "analysis",
	"content_script_status", "target_context",
	"message", "hint", "retry", "retryable", "csp_blocked", "failure_cause", "error_code",
	"terminal_reason",
	"matched", "candidates", "match_count", "match_strategy",
	"viewport",
}

func EnrichCommandResponseData(result json.RawMessage, responseData map[string]any, corrID ...string) (embeddedErr string, hasEmbeddedErr bool) {
	if len(result) == 0 {
		return "", false
	}

	var extResult map[string]any
	if err := json.Unmarshal(result, &extResult); err != nil {
		return "", false
	}

	surfaceEnrichedFields(extResult, responseData)
	surfaceTabContext(extResult, responseData)

	cid := ""
	if len(corrID) > 0 {
		cid = corrID[0]
	}
	surfaceReturnValue(extResult, responseData, cid)

	return embeddedFailure(extResult)
}

// surfaceEnrichedFields promotes the extension's enrichment fields to the
// top-level response for easier LLM consumption. Non-tab fields are always surfaced.
func surfaceEnrichedFields(extResult, responseData map[string]any) {
	for _, key := range enrichedFieldKeys {
		if v, ok := extResult[key]; ok {
			responseData[key] = v
		}
	}
}

// surfaceTabContext deduplicates tab context: effective_* fields are always
// surfaced, while resolved_*/final_url/title only appear when the URL changed
// (navigation/redirect).
func surfaceTabContext(extResult, responseData map[string]any) {
	resolvedURL, _ := extResult["resolved_url"].(string)
	effectiveURL, _ := extResult["effective_url"].(string)
	effectiveTitle, _ := extResult["effective_title"].(string)

	if effectiveURL != "" {
		responseData["effective_url"] = effectiveURL
	}
	if v, ok := extResult["effective_tab_id"]; ok {
		responseData["effective_tab_id"] = v
	}
	if effectiveTitle != "" {
		responseData["effective_title"] = effectiveTitle
	}

	if resolvedURL != "" && effectiveURL != "" && resolvedURL != effectiveURL {
		responseData["resolved_tab_id"] = extResult["resolved_tab_id"]
		responseData["resolved_url"] = resolvedURL
		if v, ok := extResult["final_url"]; ok {
			responseData["final_url"] = v
		}
		if v, ok := extResult["title"]; ok {
			responseData["title"] = v
		}
		responseData["tab_changed"] = true
		responseData["navigation_detected"] = true
		responseData["navigation_note"] = fmt.Sprintf("Page navigated from %s to %s", resolvedURL, effectiveURL)
	}
}

// surfaceReturnValue promotes an execute_js return value prominently so agents
// don't have to dig into result.result. The field is named "return_value" to
// distinguish the script's return value from the overall command result envelope.
// Only applies to execute_js commands (corrID prefix "exec_") to avoid leaking
// internal result fields from other action types.
func surfaceReturnValue(extResult, responseData map[string]any, cid string) {
	if v, ok := extResult["result"]; ok && strings.HasPrefix(cid, "exec_") {
		responseData["return_value"] = v
	}
}

// embeddedFailure reports whether the extension result carries an embedded
// failure and extracts its human-readable message.
func embeddedFailure(extResult map[string]any) (string, bool) {
	if success, ok := extResult["success"].(bool); ok && !success {
		msg := embeddedCommandError(extResult)
		if msg == "" {
			msg = "Command reported success=false"
		}
		return msg, true
	}

	if _, ok := extResult["error"]; ok {
		if msg := embeddedCommandError(extResult); msg != "" {
			return msg, true
		}
	}

	return "", false
}

func embeddedCommandError(extResult map[string]any) string {
	if msg, ok := extResult["error"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := extResult["message"].(string); ok && msg != "" {
		return msg
	}
	return ""
}
