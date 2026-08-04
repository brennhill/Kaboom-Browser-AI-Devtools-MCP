// redaction_engine.go — Owns compiled patterns, configuration, and string redaction.
package redaction

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

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
	hints       []string
	foldHints   bool
	minDigits   int
	fastReplace func(string, string) string
}

// RedactionEngine applies a set of compiled patterns to text.
// It is safe for concurrent use after construction.
type RedactionEngine struct {
	patterns []compiledPattern
}

// builtinPatterns defines the always-active redaction rules.
var builtinPatterns = []struct {
	name        string
	pattern     string
	validate    func(string) bool
	hints       []string
	foldHints   bool
	minDigits   int
	fastReplace func(string, string) string
}{
	{
		name:        "aws-key",
		pattern:     `AKIA[0-9A-Z]{16}`,
		hints:       []string{"AKIA"},
		fastReplace: redactAWSKey,
	},
	{
		name:    "bearer-token",
		pattern: `(?i)Bearer[ \t]+[A-Za-z0-9\-._~+/]+=*`,
		hints:   []string{"bearer"}, foldHints: true, fastReplace: redactBearerToken,
	},
	{
		name:    "basic-auth",
		pattern: `(?i)Basic[ \t]+[A-Za-z0-9+/]+=*`,
		hints:   []string{"basic"}, foldHints: true,
	},
	{
		name:    "jwt",
		pattern: `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+`,
		hints:   []string{"eyJ"},
	},
	{
		name:    "github-pat",
		pattern: `(ghp_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{36,})`,
		hints:   []string{"ghp_", "github_pat_"},
	},
	{
		name:    "private-key",
		pattern: `-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`,
		hints:   []string{"-----BEGIN "},
	},
	{
		name:      "credit-card",
		pattern:   `\b([0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4})\b`,
		validate:  luhnValidateMatch,
		minDigits: 16,
	},
	{
		name:        "ssn",
		pattern:     `\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`,
		minDigits:   9,
		fastReplace: redactSSN,
	},
	{
		name:    "api-key",
		pattern: `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+`,
		hints:   []string{"key"}, foldHints: true,
	},
	{
		name:    "session-cookie",
		pattern: `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}`,
		hints:   []string{"session", "sid", "token"}, foldHints: true,
	},
	{
		name:    "openai-key",
		pattern: `sk-[A-Za-z0-9_-]{16,}`,
		hints:   []string{"sk-"},
	},
	{
		name:    "slack-token",
		pattern: `xox[baprs]-[A-Za-z0-9-]{10,}`,
		hints:   []string{"xox"},
	},
}

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

// luhnValid checks if a numeric string passes the Luhn algorithm.
func luhnValid(number string) bool {
	// Strip non-digit characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)

	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// luhnValidateMatch is the validation function used by the credit-card pattern.
func luhnValidateMatch(match string) bool {
	return luhnValid(match)
}
