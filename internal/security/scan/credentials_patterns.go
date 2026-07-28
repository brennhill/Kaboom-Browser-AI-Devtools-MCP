// credentials_patterns.go — Credential regex catalogue, redaction and log-entry helpers.
// Purpose: Defines compiled regex patterns for detecting credentials in network traffic.
// Why: Centralizes credential pattern definitions separate from scanning and helper logic.
package scan

import (
	"regexp"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

var (
	// Credential patterns compiled once
	awsKeyPattern      = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	githubTokenPattern = regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`)
	stripeKeyPattern   = regexp.MustCompile(`sk_(test|live)_[A-Za-z0-9]{24,}`)
	jwtPattern         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)
	privateKeyPattern  = regexp.MustCompile(`-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----`)
	apiKeyURLPattern   = regexp.MustCompile(`(?i)[?&](api[_-]?key|apikey|key|token|secret|password|passwd|api_secret)=([^&]{8,})`)
	bearerPattern      = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]{20,}`)
	apiKeyBodyPattern  = regexp.MustCompile(`(?i)"(api[_-]?key|apiKey|api_secret|apiSecret)":\s*"([^"]{8,})"`)
	genericSecretURL   = regexp.MustCompile(`(?i)[?&]\w*(secret|password|passwd|token)\w*=([^&]{8,})`)

	// Test/dev key indicators — reduces severity
	testKeyIndicators = []string{"test", "dev", "example", "sample", "demo", "dummy", "fake", "mock"}
)

// credentialPatternCheck defines a single credential pattern to scan for in body content.
type credentialPatternCheck struct {
	pattern     *regexp.Regexp
	severity    string
	titleFmt    string
	descFmt     string
	remediation string
	evidence    string // if non-empty, use this instead of redacted match
	skipTestKey bool
}

// bodyCredentialChecks returns the ordered list of credential patterns to scan.
func bodyCredentialChecks() []credentialPatternCheck {
	return []credentialPatternCheck{
		{awsKeyPattern, "critical", "AWS access key in %s", "An AWS access key ID pattern was detected in the %s.", "Remove AWS credentials from API responses. Use short-lived STS tokens if needed.", "", true},
		{githubTokenPattern, "critical", "GitHub token in %s", "A GitHub personal access token was detected in the %s.", "Remove GitHub tokens from client-visible responses. Use short-lived tokens.", "", true},
		{stripeKeyPattern, "critical", "Stripe secret key in %s", "A Stripe secret key was detected in the %s. This key can be used to make charges.", "Never expose Stripe secret keys to the client. Use publishable keys (pk_*) for client-side operations.", "", true},
		{privateKeyPattern, "critical", "Private key material in %s", "Private key material was detected in the %s. This is a critical exposure.", "Never transmit private keys in API responses. Use key management services.", "-----BEGIN ... PRIVATE KEY-----", false},
		{jwtPattern, "medium", "JWT token in %s", "A JWT token was detected in the %s. Verify this is an intentional auth token delivery.", "Ensure JWT tokens are only delivered via secure, intended channels (e.g., httpOnly cookies).", "", false},
	}
}

// redactSecret masks a secret value, showing only the first 6 and last 3 characters.
func redactSecret(s string) string {
	if len(s) <= 6 {
		if len(s) <= 3 {
			return s + "***"
		}
		return s[:3] + "***"
	}
	if len(s) <= 10 {
		return s[:6] + "***"
	}
	return s[:6] + "***" + s[len(s)-3:]
}

func isTestKey(value string) bool {
	lower := strings.ToLower(value)
	for _, indicator := range testKeyIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func getEntryString(entry types.LogEntry, key string) string {
	val, ok := entry[key]
	if !ok || val == nil {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}
