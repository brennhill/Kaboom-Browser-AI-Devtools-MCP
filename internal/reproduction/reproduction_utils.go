// Purpose: Provides JavaScript string escaping and URL rewriting utilities for reproduction scripts.
// Why: Centralizes shared utilities used by both Kaboom and Playwright reproduction generators.
package reproduction

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/url"
	"strings"
)

// EscapeJS escapes a string for embedding in JavaScript string literals.
func EscapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// rewriteURL replaces the origin of a URL with baseURL.
func rewriteURL(originalURL, baseURL string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	return strings.TrimRight(baseURL, "/") + parsed.Path
}

// ChopString truncates s to maxLen characters.
func ChopString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// collectSelectorTypes returns the unique selector types present across actions.
func collectSelectorTypes(actions []types.EnhancedAction) []string {
	types := make(map[string]bool)
	for _, a := range actions {
		if a.Selectors == nil {
			continue
		}
		for key := range a.Selectors {
			types[key] = true
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	return result
}
