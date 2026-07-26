// Purpose: Detects fragile selectors by analyzing failure rates across multiple playback sessions.
// Why: Isolates selector reliability analysis from action execution and session management.
package playback

import "fmt"

// DetectFragileSelectors flags selectors that failed in more than half of their
// click attempts across two or more sessions. A single session is not enough
// evidence and always yields an empty result.
func DetectFragileSelectors(sessions []*Session) map[string]bool {
	fragile := make(map[string]bool)
	if len(sessions) < 2 {
		return fragile
	}

	selectorRunCount := make(map[string]int)
	selectorFailCount := make(map[string]int)

	for _, session := range sessions {
		for _, result := range session.Results {
			if result.ActionType == "click" && result.SelectorUsed != "" {
				key := fmt.Sprintf("%s:%s", result.SelectorUsed, result.SelectorUsed)
				selectorRunCount[key]++
				if result.Status == "error" {
					selectorFailCount[key]++
				}
			}
		}
	}

	for selector, runCount := range selectorRunCount {
		failureRate := float64(selectorFailCount[selector]) / float64(runCount)
		if failureRate > 0.5 {
			fragile[selector] = true
		}
	}

	return fragile
}
