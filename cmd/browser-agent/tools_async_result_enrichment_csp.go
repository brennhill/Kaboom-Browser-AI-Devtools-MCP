// Purpose: Detects CSP-related failures and enriches async command responses with CSP guidance.
// Why: Keeps CSP heuristics and retry annotation logic separate from generic result enrichment.

package main

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

func annotateCSPFailure(responseData map[string]any, cmdError string, result json.RawMessage) {
	cspBlocked, errorCode, message := detectCSPFailure(cmdError, result)
	if !cspBlocked {
		return
	}

	responseData["csp_blocked"] = true
	responseData["failure_cause"] = "csp"

	if errorCode != "" {
		responseData["error_code"] = errorCode
	}
	if message != "" {
		if _, exists := responseData["message"]; !exists {
			responseData["message"] = message
		}
	}
	if _, exists := responseData["retry"]; !exists {
		responseData["retry"] = configurehandler.CSPRetryNavigationGuidance
	}
}

func detectCSPFailure(cmdError string, result json.RawMessage) (bool, string, string) {
	errorCode := ""
	message := ""

	if len(result) > 0 {
		var extResult map[string]any
		if err := json.Unmarshal(result, &extResult); err == nil {
			if v, ok := extResult["error"].(string); ok {
				errorCode = strings.TrimSpace(v)
			}
			if v, ok := extResult["message"].(string); ok {
				message = strings.TrimSpace(v)
			}
			if v, ok := extResult["csp_blocked"].(bool); ok && v {
				return true, errorCode, message
			}
			if v, ok := extResult["failure_cause"].(string); ok && strings.EqualFold(strings.TrimSpace(v), "csp") {
				return true, errorCode, message
			}
			for _, key := range []string{"error", "message", "hint"} {
				if v, ok := extResult[key].(string); ok && looksLikeCSP(v) {
					return true, errorCode, message
				}
			}
		}
	}

	if looksLikeCSP(cmdError) {
		if errorCode == "" {
			errorCode = strings.TrimSpace(cmdError)
		}
		return true, errorCode, message
	}

	return false, errorCode, message
}

func looksLikeCSP(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return false
	}
	return strings.Contains(v, "csp") ||
		strings.Contains(v, "content security policy") ||
		strings.Contains(v, "trusted type") ||
		strings.Contains(v, "unsafe-eval") ||
		strings.Contains(v, "blocks content scripts") ||
		strings.Contains(v, "blocked content scripts") ||
		strings.Contains(v, "restricted_page")
}
