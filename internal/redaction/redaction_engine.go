// Purpose: Creates and runs the redaction engine, compiling built-in and custom regex patterns.
// Why: Separates engine construction and string redaction from pattern definitions and key matching.
package redaction

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

var (
	builtinEngineOnce            sync.Once
	builtinEngine                *RedactionEngine
	diagnosticQuotedCredential   = regexp.MustCompile(`(?i)(["']?(?:token|secret|password|api[_-]?key|authorization)["']?\s*[:=]\s*)["'][^"'\r\n]*["']`)
	diagnosticUnquotedCredential = regexp.MustCompile(`(?i)["']?(?:token|secret|password|api[_-]?key|authorization)["']?\s*[:=]\s*[^\s,;&}{]+`)
)

// RedactSensitiveText applies the canonical built-in secret patterns without
// loading project configuration. Operational diagnostics use this boundary so
// they cannot accidentally depend on workspace-specific redaction settings.
func RedactSensitiveText(input string) string {
	builtinEngineOnce.Do(func() { builtinEngine = NewRedactionEngine("") })
	builtin := builtinEngine.Redact(input)
	quoted := diagnosticQuotedCredential.ReplaceAllString(builtin, "$1\"[REDACTED:structured-credential]\"")
	return diagnosticUnquotedCredential.ReplaceAllString(quoted, "[REDACTED:structured-credential]")
}

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
			hints:       bp.hints,
			foldHints:   bp.foldHints,
			minDigits:   bp.minDigits,
			fastReplace: bp.fastReplace,
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
		if !patternMayMatch(result, p) {
			continue
		}
		if p.fastReplace != nil {
			result = p.fastReplace(result, p.replacement)
			continue
		}
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

func redactSSN(input, replacement string) string {
	var output strings.Builder
	last := 0
	for start := 0; start+11 <= len(input); start++ {
		if !isSSNAt(input, start) {
			continue
		}
		if output.Cap() == 0 {
			output.Grow(len(input))
		}
		output.WriteString(input[last:start])
		output.WriteString(replacement)
		last = start + 11
		start += 10
	}
	if last == 0 {
		return input
	}
	output.WriteString(input[last:])
	return output.String()
}

func redactAWSKey(input, replacement string) string {
	return replaceFixedCandidates(input, replacement, 20, func(candidate string) bool {
		if !strings.HasPrefix(candidate, "AKIA") {
			return false
		}
		for index := 4; index < len(candidate); index++ {
			char := candidate[index]
			if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
				return false
			}
		}
		return true
	})
}

func redactBearerToken(input, replacement string) string {
	var output strings.Builder
	last := 0
	for start := 0; start+7 <= len(input); start++ {
		if !equalASCIIFold(input[start:start+6], "bearer") || (input[start+6] != ' ' && input[start+6] != '\t') {
			continue
		}
		end := start + 6
		for end < len(input) && (input[end] == ' ' || input[end] == '\t') {
			end++
		}
		tokenStart := end
		for end < len(input) && isBearerBaseChar(input[end]) {
			end++
		}
		if end == tokenStart {
			continue
		}
		for end < len(input) && input[end] == '=' {
			end++
		}
		if output.Cap() == 0 {
			output.Grow(len(input))
		}
		output.WriteString(input[last:start])
		output.WriteString(replacement)
		last = end
		start = end - 1
	}
	if last == 0 {
		return input
	}
	output.WriteString(input[last:])
	return output.String()
}

func replaceFixedCandidates(input, replacement string, width int, valid func(string) bool) string {
	var output strings.Builder
	last := 0
	for start := 0; start+width <= len(input); start++ {
		if !valid(input[start : start+width]) {
			continue
		}
		if output.Cap() == 0 {
			output.Grow(len(input))
		}
		output.WriteString(input[last:start])
		output.WriteString(replacement)
		last = start + width
		start += width - 1
	}
	if last == 0 {
		return input
	}
	output.WriteString(input[last:])
	return output.String()
}

func equalASCIIFold(input, lower string) bool {
	if len(input) != len(lower) {
		return false
	}
	for index := range len(lower) {
		char := input[index]
		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}
		if char != lower[index] {
			return false
		}
	}
	return true
}

func isBearerBaseChar(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-._~+/", rune(char))
}

func isSSNAt(input string, start int) bool {
	if start > 0 && isASCIIWord(input[start-1]) || start+11 < len(input) && isASCIIWord(input[start+11]) {
		return false
	}
	for _, offset := range [...]int{0, 1, 2, 4, 5, 7, 8, 9, 10} {
		if input[start+offset] < '0' || input[start+offset] > '9' {
			return false
		}
	}
	return input[start+3] == '-' && input[start+6] == '-'
}

func isASCIIWord(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_'
}

func patternMayMatch(input string, pattern compiledPattern) bool {
	if pattern.minDigits > 0 && !containsAtLeastDigits(input, pattern.minDigits) {
		return false
	}
	if len(pattern.hints) == 0 {
		return true
	}
	for _, hint := range pattern.hints {
		if (!pattern.foldHints && strings.Contains(input, hint)) || (pattern.foldHints && containsASCIIFold(input, hint)) {
			return true
		}
	}
	return false
}

func containsAtLeastDigits(input string, minimum int) bool {
	count := 0
	for index := range len(input) {
		if input[index] >= '0' && input[index] <= '9' {
			count++
			if count >= minimum {
				return true
			}
		}
	}
	return false
}

func containsASCIIFold(input, lowerHint string) bool {
	if len(lowerHint) == 0 {
		return true
	}
	for start := 0; start+len(lowerHint) <= len(input); start++ {
		matched := true
		for offset := range len(lowerHint) {
			char := input[start+offset]
			if char >= 'A' && char <= 'Z' {
				char += 'a' - 'A'
			}
			if char != lowerHint[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
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
