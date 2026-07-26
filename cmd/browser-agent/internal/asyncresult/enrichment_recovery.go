// enrichment_recovery.go — Detects interact failure codes and annotates command results with recovery playbooks.
// Why: Isolates recovery-specific enrichment logic from generic async response promotion.

package asyncresult

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
)

func AnnotateInteractFailureRecovery(responseData map[string]any, cmdError string, result json.RawMessage) {
	code := detectInteractFailureCode(responseData, cmdError, result)
	canonical, playbook, ok := playbooks.LookupInteractFailurePlaybook(code)
	if !ok {
		return
	}
	if _, exists := responseData["error_code"]; !exists {
		responseData["error_code"] = canonical
	}
	if _, exists := responseData["retry"]; !exists {
		responseData["retry"] = playbook.RetrySuggestion
	}
	if _, exists := responseData["hint"]; !exists {
		responseData["hint"] = "Recovery playbook available for " + canonical + " in configure tutorial."
	}
	if _, exists := responseData["retryable"]; !exists {
		responseData["retryable"] = true
	}

	// For ambiguous_target: compute suggested_element_id from first visible candidate.
	if canonical == "ambiguous_target" {
		annotateSuggestedElementID(responseData)
	}
}

// annotateSuggestedElementID picks the first visible candidate's element_id
// and sets it as suggested_element_id for single-retry LLM recovery.
func annotateSuggestedElementID(responseData map[string]any) {
	// Skip if extension already set suggested_element_id (e.g. ranked resolution).
	if existing, ok := responseData["suggested_element_id"].(string); ok && existing != "" {
		return
	}
	candidates, ok := responseData["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return
	}
	for _, c := range candidates {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		visible, _ := cMap["visible"].(bool)
		elementID, _ := cMap["element_id"].(string)
		if visible && elementID != "" {
			responseData["suggested_element_id"] = elementID
			return
		}
	}
}

func detectInteractFailureCode(responseData map[string]any, cmdError string, result json.RawMessage) string {
	candidates := make([]string, 0, 4)
	if v, ok := responseData["error_code"].(string); ok && strings.TrimSpace(v) != "" {
		candidates = append(candidates, v)
	}
	if v, ok := responseData["error"].(string); ok && strings.TrimSpace(v) != "" {
		candidates = append(candidates, v)
	}

	if len(result) > 0 {
		var extResult map[string]any
		if err := json.Unmarshal(result, &extResult); err == nil {
			if v, ok := extResult["error"].(string); ok && strings.TrimSpace(v) != "" {
				candidates = append(candidates, v)
			}
			if v, ok := extResult["error_code"].(string); ok && strings.TrimSpace(v) != "" {
				candidates = append(candidates, v)
			}
		}
	}

	if strings.TrimSpace(cmdError) != "" {
		candidates = append(candidates, cmdError)
	}

	for _, candidate := range candidates {
		if normalized := playbooks.NormalizeInteractFailureCode(candidate); normalized != "" {
			return normalized
		}
	}
	return ""
}
