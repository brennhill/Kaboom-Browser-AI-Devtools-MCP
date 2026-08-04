// Purpose: Defines always-active regex patterns for redacting secrets (AWS keys, tokens, private keys).
// Why: Centralizes built-in credential patterns separate from engine initialization and key-based redaction.
package redaction

// builtinPatterns defines the always-active redaction rules.
var builtinPatterns = []struct {
	name     string
	pattern  string
	validate func(string) bool
}{
	{
		name:    "aws-key",
		pattern: `AKIA[0-9A-Z]{16}`,
	},
	{
		name:    "bearer-token",
		pattern: `(?i)Bearer[ \t]+[A-Za-z0-9\-._~+/]+=*`,
	},
	{
		name:    "basic-auth",
		pattern: `(?i)Basic[ \t]+[A-Za-z0-9+/]+=*`,
	},
	{
		name:    "jwt",
		pattern: `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+`,
	},
	{
		name:    "github-pat",
		pattern: `(ghp_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{36,})`,
	},
	{
		name:    "private-key",
		pattern: `-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`,
	},
	{
		name:     "credit-card",
		pattern:  `\b([0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4})\b`,
		validate: luhnValidateMatch,
	},
	{
		name:    "ssn",
		pattern: `\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`,
	},
	{
		name:    "api-key",
		pattern: `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+`,
	},
	{
		name:    "session-cookie",
		pattern: `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}`,
	},
	{
		name:    "openai-key",
		pattern: `sk-[A-Za-z0-9_-]{16,}`,
	},
	{
		name:    "slack-token",
		pattern: `xox[baprs]-[A-Za-z0-9-]{10,}`,
	},
}
