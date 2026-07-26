// credentials.go — URL, body and console credential scanning.
// Purpose: Scans network bodies and console logs for exposed credentials using pattern matching.
// Why: Separates credential scanning logic from pattern definitions and helper utilities.
package scan

import (
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func (s *Scanner) checkCredentials(bodies []capture.NetworkBody, entries []LogEntry) []Finding {
	var findings []Finding

	// Scan network bodies (URLs and body content)
	for _, body := range bodies {
		findings = append(findings, s.scanURLForCredentials(body)...)
		findings = append(findings, s.scanBodyForCredentials(body.RequestBody, body.URL, "request body")...)
		findings = append(findings, s.scanBodyForCredentials(body.ResponseBody, body.URL, "response body")...)
	}

	// Scan console entries
	for _, entry := range entries {
		findings = append(findings, s.scanConsoleForCredentials(entry)...)
	}

	return findings
}

// scanURLForAPIKeys checks for API keys in URL query parameters.
func (s *Scanner) scanURLForAPIKeys(url string) []Finding {
	var findings []Finding
	matches := apiKeyURLPattern.FindAllStringSubmatch(url, 10)
	for _, m := range matches {
		if len(m) < 3 || isTestKey(m[2]) {
			continue
		}
		findings = append(findings, Finding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       fmt.Sprintf("API key exposed in URL query parameter '%s'", m[1]),
			Description: fmt.Sprintf("GET request includes secret in query parameter '%s'. Query parameters are logged in server access logs, browser history, and may be cached by proxies.", m[1]),
			Location:    url,
			Evidence:    redactSecret(m[2]),
			Remediation: "Move API key to Authorization header or request body. Never include secrets in URLs.",
		})
	}
	return findings
}

// scanURLForGenericSecrets checks for generic secret parameters in URL.
func (s *Scanner) scanURLForGenericSecrets(url string) []Finding {
	if apiKeyURLPattern.MatchString(url) {
		return nil // avoid duplicating apiKey findings
	}
	var findings []Finding
	matches := genericSecretURL.FindAllStringSubmatch(url, 10)
	for _, m := range matches {
		if len(m) < 3 || isTestKey(m[2]) {
			continue
		}
		findings = append(findings, Finding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "Secret value exposed in URL query parameter",
			Description: "Request URL contains a secret-named parameter with a long value.",
			Location:    url,
			Evidence:    redactSecret(m[2]),
			Remediation: "Move secrets to Authorization header or request body.",
		})
	}
	return findings
}

func (s *Scanner) scanURLForCredentials(body capture.NetworkBody) []Finding {
	var findings []Finding
	findings = append(findings, s.scanURLForAPIKeys(body.URL)...)
	findings = append(findings, s.scanURLForGenericSecrets(body.URL)...)

	if jwtPattern.MatchString(body.URL) {
		findings = append(findings, Finding{
			Check: "credentials", Severity: "critical",
			Title:       "JWT token exposed in URL",
			Description: "A JWT token was found in the request URL. URLs are logged in browser history, server logs, and may leak via Referer headers.",
			Location:    body.URL, Evidence: redactSecret(jwtPattern.FindString(body.URL)),
			Remediation: "Pass JWT tokens in the Authorization header, not in URLs.",
		})
	}
	if awsKeyPattern.MatchString(body.URL) {
		findings = append(findings, Finding{
			Check: "credentials", Severity: "critical",
			Title:       "AWS access key exposed in URL",
			Description: "An AWS access key ID was found in the request URL.",
			Location:    body.URL, Evidence: redactSecret(awsKeyPattern.FindString(body.URL)),
			Remediation: "Use IAM roles or environment variables for AWS credentials. Never embed in URLs.",
		})
	}
	return findings
}

func (s *Scanner) scanBodyForCredentials(bodyContent, sourceURL, location string) []Finding {
	if bodyContent == "" {
		return nil
	}
	scanContent := bodyContent
	if len(scanContent) > 10240 {
		scanContent = scanContent[:10240]
	}

	var findings []Finding
	for _, check := range bodyCredentialChecks() {
		if !check.pattern.MatchString(scanContent) {
			continue
		}
		match := check.pattern.FindString(scanContent)
		if check.skipTestKey && isTestKey(match) {
			continue
		}
		evidence := check.evidence
		if evidence == "" {
			evidence = redactSecret(match)
		}
		findings = append(findings, Finding{
			Check:       "credentials",
			Severity:    check.severity,
			Title:       fmt.Sprintf(check.titleFmt, location),
			Description: fmt.Sprintf(check.descFmt, location),
			Location:    sourceURL,
			Evidence:    evidence,
			Remediation: check.remediation,
		})
	}

	// API key in JSON body (multi-match pattern)
	for _, m := range apiKeyBodyPattern.FindAllStringSubmatch(scanContent, 5) {
		if len(m) < 3 || isTestKey(m[2]) {
			continue
		}
		findings = append(findings, Finding{
			Check:       "credentials",
			Severity:    "warning",
			Title:       fmt.Sprintf("API key '%s' in %s", m[1], location),
			Description: fmt.Sprintf("An API key field '%s' was found in the %s.", m[1], location),
			Location:    sourceURL,
			Evidence:    redactSecret(m[2]),
			Remediation: "Verify this key is meant to be client-visible. Use server-side proxy for secret keys.",
		})
	}

	return findings
}

func (s *Scanner) scanConsoleForCredentials(entry LogEntry) []Finding {
	var findings []Finding

	msg := getEntryString(entry, "message")
	if msg == "" {
		return findings
	}

	// Limit scan depth
	if len(msg) > 10240 {
		msg = msg[:10240]
	}

	source := getEntryString(entry, "source")

	// Check for Bearer tokens
	if bearerPattern.MatchString(msg) {
		match := bearerPattern.FindString(msg)
		findings = append(findings, Finding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "Bearer token logged to console",
			Description: "A Bearer token was found in console output. Console logs may be captured by browser extensions or error tracking services.",
			Location:    source,
			Evidence:    redactSecret(match),
			Remediation: "Remove console.log statements that output tokens. Use structured logging with redaction.",
		})
	}

	// Check for JWT
	if jwtPattern.MatchString(msg) {
		match := jwtPattern.FindString(msg)
		// Don't double-count if already caught by bearer check
		if !bearerPattern.MatchString(msg) {
			findings = append(findings, Finding{
				Check:       "credentials",
				Severity:    "critical",
				Title:       "JWT token logged to console",
				Description: "A JWT token was found in console output.",
				Location:    source,
				Evidence:    redactSecret(match),
				Remediation: "Remove console.log statements that output tokens.",
			})
		}
	}

	// AWS key in console
	if awsKeyPattern.MatchString(msg) {
		match := awsKeyPattern.FindString(msg)
		findings = append(findings, Finding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "AWS access key logged to console",
			Description: "An AWS access key was found in console output.",
			Location:    source,
			Evidence:    redactSecret(match),
			Remediation: "Never log AWS credentials. Use environment variables and IAM roles.",
		})
	}

	return findings
}
