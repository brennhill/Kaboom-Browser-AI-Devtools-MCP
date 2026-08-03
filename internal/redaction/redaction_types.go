// Purpose: Defines redaction pattern configuration and compiled matcher state.
// Why: Keeps redaction-specific types local while MCP wire types remain canonical in internal/mcp.
package redaction

import "regexp"

// RedactionPattern represents a single redaction rule.
type RedactionPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement,omitempty"`
}

// RedactionConfig represents the JSON configuration file structure.
type RedactionConfig struct {
	Patterns []RedactionPattern `json:"patterns"`
}

// compiledPattern holds a pre-compiled regex and its replacement string.
type compiledPattern struct {
	name        string
	regex       *regexp.Regexp
	replacement string
	validate    func(match string) bool // optional post-match validation (e.g., Luhn)
}

// RedactionEngine applies a set of compiled patterns to text.
// It is safe for concurrent use after construction.
type RedactionEngine struct {
	patterns []compiledPattern
}
