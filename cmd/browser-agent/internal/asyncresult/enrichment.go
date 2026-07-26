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

	// Surface extension enrichment fields to top-level for easier LLM consumption.
	// Non-tab fields are always surfaced.
	for _, key := range enrichedFieldKeys {
		if v, ok := extResult[key]; ok {
			responseData[key] = v
		}
	}

	// Deduplicate tab context: only include resolved_*/final_url/title when URLs changed.
	resolvedURL, _ := extResult["resolved_url"].(string)
	effectiveURL, _ := extResult["effective_url"].(string)
	effectiveTitle, _ := extResult["effective_title"].(string)

	// Always surface effective_* when present.
	if effectiveURL != "" {
		responseData["effective_url"] = effectiveURL
	}
	if v, ok := extResult["effective_tab_id"]; ok {
		responseData["effective_tab_id"] = v
	}
	if effectiveTitle != "" {
		responseData["effective_title"] = effectiveTitle
	}

	// Only surface resolved_*/final_url/title when URL changed (navigation/redirect).
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

	// Surface execute_js return values prominently so agents don't have to
	// dig into result.result. The field is named "return_value" to distinguish
	// the script's return value from the overall command result envelope.
	// Only applies to execute_js commands (corrID prefix "exec_") to avoid
	// leaking internal result fields from other action types.
	cid := ""
	if len(corrID) > 0 {
		cid = corrID[0]
	}
	if v, ok := extResult["result"]; ok && strings.HasPrefix(cid, "exec_") {
		responseData["return_value"] = v
	}

	if success, ok := extResult["success"].(bool); ok && !success {
		msg := embeddedCommandError(extResult)
		if msg == "" {
			msg = "Command reported success=false"
		}
		return msg, true
	}

	if _, ok := extResult["error"]; ok {
		msg := embeddedCommandError(extResult)
		if msg != "" {
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
