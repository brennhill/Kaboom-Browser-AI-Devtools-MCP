// Purpose: Creates and runs the redaction engine, compiling built-in and custom regex patterns.
// Why: Separates engine construction and string redaction from pattern definitions and key matching.
package redaction

import (
	"encoding/json"
	"os"
	"regexp"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// NewRedactionEngine creates a new engine with built-in patterns and optional
// custom patterns loaded from the given config file path.
// If configPath is empty or the file cannot be read, only built-in patterns are used.
// Invalid regex patterns in the config file are skipped silently.
func NewRedactionEngine(configPath string) *RedactionEngine {
	engine := &RedactionEngine{}

	// Compile built-in patterns
	for _, bp := range builtinPatterns {
		re, err := regexp.Compile(bp.pattern)
		if err != nil {
			continue // should never happen for built-ins, but be safe
		}
		engine.patterns = append(engine.patterns, compiledPattern{
			name:        bp.name,
			regex:       re,
			replacement: "[REDACTED:" + bp.name + "]",
			validate:    bp.validate,
		})
	}

	// Load custom patterns from config file
	if configPath != "" {
		engine.loadConfig(configPath)
	}

	return engine
}

// loadConfig reads and parses the JSON config file, compiling valid patterns.
func (e *RedactionEngine) loadConfig(path string) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is from trusted config location
	if err != nil {
		return // file not found or unreadable — use built-ins only
	}

	var config RedactionConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return // invalid JSON — use built-ins only
	}

	for _, p := range config.Patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			continue // skip invalid regex (e.g., PCRE-only features)
		}

		replacement := p.Replacement
		if replacement == "" {
			replacement = "[REDACTED:" + p.Name + "]"
		}

		e.patterns = append(e.patterns, compiledPattern{
			name:        p.Name,
			regex:       re,
			replacement: replacement,
		})
	}
}

// Redact applies all patterns to the input string and returns the redacted result.
// Thread-safe: compiled regexps in Go are safe for concurrent use.
func (e *RedactionEngine) Redact(input string) string {
	if input == "" {
		return ""
	}

	result := input
	for _, p := range e.patterns {
		if p.validate != nil {
			// For patterns with validation, we need to check each match
			result = p.regex.ReplaceAllStringFunc(result, func(match string) string {
				if p.validate(match) {
					return p.replacement
				}
				return match
			})
		} else {
			result = p.regex.ReplaceAllString(result, p.replacement)
		}
	}
	return result
}

// RedactJSON applies redaction to every string-bearing field within a canonical
// MCP tool result, including structured text, metadata, and malformed image fields.
// If the JSON is malformed, returns the input with string-level redaction applied.
func (e *RedactionEngine) RedactJSON(input json.RawMessage) json.RawMessage {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(input, &result); err != nil {
		// Fallback: redact the raw JSON string
		redacted := e.Redact(string(input))
		return json.RawMessage(redacted)
	}

	// Canonical text blocks may themselves contain structured JSON. Image data is
	// normally base64 and remains unchanged, while malformed raw credentials are
	// still removed instead of being trusted merely because the block says image.
	for i := range result.Content {
		result.Content[i].Text = e.redactStructuredText(result.Content[i].Text)
		result.Content[i].Data = e.Redact(result.Content[i].Data)
		result.Content[i].MimeType = e.Redact(result.Content[i].MimeType)
	}
	result.Metadata = e.RedactMapValues(result.Metadata)

	output, err := json.Marshal(result)
	if err != nil {
		// Should never happen, but fallback to raw redaction
		return json.RawMessage(e.Redact(string(input)))
	}
	return json.RawMessage(output)
}

func (e *RedactionEngine) redactStructuredText(input string) string {
	var value any
	if json.Unmarshal([]byte(input), &value) != nil {
		return e.Redact(input)
	}
	redacted, err := json.Marshal(e.redactValue("", value))
	if err != nil {
		return e.Redact(input)
	}
	return string(redacted)
}
