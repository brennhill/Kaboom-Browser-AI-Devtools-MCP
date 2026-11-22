// Purpose: Applies regex-based redaction patterns to audit entry parameters before storage.
// Why: Isolates sensitive-data scrubbing from audit recording so redaction rules can evolve independently.
package audit

import "regexp"

// redactParameters applies all configured redaction patterns to the parameter string.
func (at *AuditTrail) redactParameters(params string) string {
	result := params
	for _, rp := range at.redactionPatterns {
		result = rp.pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// compileRedactionPatterns returns the built-in redaction patterns compiled.
func compileRedactionPatterns() []*redactionPattern {
	patterns := []struct {
		name    string
		pattern string
	}{
		// Bearer tokens (OAuth)
		{"bearer_token", `Bearer\s+[A-Za-z0-9\-._~+/]+=*`},
		// API keys in key=value format
		{"api_key", `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+`},
		// JSON Web Tokens
		{"jwt", `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`},
		// Session cookies/tokens (long random values)
		{"session_cookie", `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}`},
	}

	compiled := make([]*redactionPattern, 0, len(patterns))
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		compiled = append(compiled, &redactionPattern{
			name:    p.name,
			pattern: re,
		})
	}

	return compiled
}
