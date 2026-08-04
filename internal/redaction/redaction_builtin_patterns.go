// Purpose: Defines always-active regex patterns for redacting secrets (AWS keys, tokens, private keys).
// Why: Centralizes built-in credential patterns separate from engine initialization and key-based redaction.
package redaction

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
