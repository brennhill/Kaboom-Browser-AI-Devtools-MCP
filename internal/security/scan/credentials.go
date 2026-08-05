// credentials.go — Credential and PII detection, redaction, and evidence shaping.
// Purpose: Owns sensitive-data detection across URLs, bodies, and console output.
// Why: Credential and PII patterns share scan limits, redaction, and release policy.
// Docs: docs/features/feature/security-hardening/index.md
package scan

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

var (
	awsKeyPattern      = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	githubTokenPattern = regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`)
	stripeKeyPattern   = regexp.MustCompile(`sk_(test|live)_[A-Za-z0-9]{24,}`)
	jwtPattern         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)
	privateKeyPattern  = regexp.MustCompile(`-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----`)
	apiKeyURLPattern   = regexp.MustCompile(`(?i)[?&](api[_-]?key|apikey|key|token|secret|password|passwd|api_secret)=([^&]{8,})`)
	bearerPattern      = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]{20,}`)
	apiKeyBodyPattern  = regexp.MustCompile(`(?i)"(api[_-]?key|apiKey|api_secret|apiSecret)":\s*"([^"]{8,})"`)
	genericSecretURL   = regexp.MustCompile(`(?i)[?&]\w*(secret|password|passwd|token)\w*=([^&]{8,})`)
	emailPattern       = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phonePattern       = regexp.MustCompile(`(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
	ssnPattern         = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ccPattern          = regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`)
)

type credentialPatternCheck struct {
	pattern     *regexp.Regexp
	severity    string
	titleFmt    string
	descFmt     string
	remediation string
	evidence    string
	skipTestKey bool
}

func bodyCredentialChecks() []credentialPatternCheck {
	return []credentialPatternCheck{
		{awsKeyPattern, "critical", "AWS access key in %s", "An AWS access key ID pattern was detected in the %s.", "Remove AWS credentials from API responses. Use short-lived STS tokens if needed.", "", true},
		{githubTokenPattern, "critical", "GitHub token in %s", "A GitHub personal access token was detected in the %s.", "Remove GitHub tokens from client-visible responses. Use short-lived tokens.", "", true},
		{stripeKeyPattern, "critical", "Stripe secret key in %s", "A Stripe secret key was detected in the %s. This key can be used to make charges.", "Never expose Stripe secret keys to the client. Use publishable keys (pk_*) for client-side operations.", "", true},
		{privateKeyPattern, "critical", "Private key material in %s", "Private key material was detected in the %s. This is a critical exposure.", "Never transmit private keys in API responses. Use key management services.", "-----BEGIN ... PRIVATE KEY-----", false},
		{jwtPattern, "medium", "JWT token in %s", "A JWT token was detected in the %s. Verify this is an intentional auth token delivery.", "Ensure JWT tokens are only delivered via secure, intended channels (e.g., httpOnly cookies).", "", false},
	}
}

func redactSecret(secret string) string {
	if len(secret) <= 6 {
		if len(secret) <= 3 {
			return secret + "***"
		}
		return secret[:3] + "***"
	}
	if len(secret) <= 10 {
		return secret[:6] + "***"
	}
	return secret[:6] + "***" + secret[len(secret)-3:]
}

func isTestKey(value string) bool {
	lower := strings.ToLower(value)
	for _, indicator := range []string{"test", "dev", "example", "sample", "demo", "dummy", "fake", "mock"} {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func getEntryString(entry types.LogEntry, key string) string {
	value, ok := entry[key]
	if !ok || value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}

func (s *Scanner) checkCredentials(bodies []types.NetworkBody, entries []types.LogEntry) []Finding {
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

func (s *Scanner) scanURLForCredentials(body types.NetworkBody) []Finding {
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

func (s *Scanner) scanConsoleForCredentials(entry types.LogEntry) []Finding {
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

func (s *Scanner) checkPII(bodies []types.NetworkBody, pageURLs []string) []Finding {
	var findings []Finding
	for _, body := range bodies {
		if body.RequestBody != "" {
			findings = append(findings, s.scanForPII(body.RequestBody, body.URL, "request body", httpsec.IsThirdPartyURL(body.URL, pageURLs))...)
		}
		if body.ResponseBody != "" {
			findings = append(findings, s.scanForPII(body.ResponseBody, body.URL, "response body", false)...)
		}
	}
	return findings
}

func scanForSSN(content, sourceURL, location string, thirdParty bool) *Finding {
	if !ssnPattern.MatchString(content) {
		return nil
	}
	severity := "high"
	description := fmt.Sprintf("A Social Security Number pattern was detected in %s.", location)
	if thirdParty {
		severity = "critical"
		description = fmt.Sprintf("A Social Security Number pattern is being sent to a third-party endpoint in %s.", location)
	}
	return &Finding{Check: "pii", Severity: severity, Title: "SSN pattern detected in " + location, Description: description, Location: sourceURL, Evidence: redactSecret(ssnPattern.FindString(content)), Remediation: "Never transmit SSNs in plain text. Use tokenization or encryption."}
}

func scanForCreditCard(content, sourceURL, location string) *Finding {
	if !ccPattern.MatchString(content) {
		return nil
	}
	match := ccPattern.FindString(content)
	cleaned := strings.ReplaceAll(strings.ReplaceAll(match, " ", ""), "-", "")
	if len(cleaned) < 13 || len(cleaned) > 19 || !looksLikeCreditCard(cleaned) {
		return nil
	}
	return &Finding{Check: "pii", Severity: "critical", Title: "Credit card number detected in " + location, Description: fmt.Sprintf("A credit card number pattern was detected in %s.", location), Location: sourceURL, Evidence: redactSecret(match), Remediation: "Never transmit full card numbers. Use tokenization (e.g., Stripe tokens)."}
}

func thirdPartySeverity(thirdParty bool) string {
	if thirdParty {
		return "warning"
	}
	return "info"
}

func scanForEmailPII(content, sourceURL, location string, thirdParty bool) *Finding {
	if !emailPattern.MatchString(content) {
		return nil
	}
	return &Finding{Check: "pii", Severity: thirdPartySeverity(thirdParty), Title: "Email address in " + location, Description: fmt.Sprintf("An email address was detected in %s.", location), Location: sourceURL, Evidence: redactSecret(emailPattern.FindString(content)), Remediation: "Review whether PII needs to be sent to this endpoint."}
}

func scanForPhonePII(content, sourceURL, location string, thirdParty bool) *Finding {
	if !phonePattern.MatchString(content) {
		return nil
	}
	match := phonePattern.FindString(content)
	if len(strings.NewReplacer("-", "", " ", "", "(", "").Replace(match)) < 10 {
		return nil
	}
	return &Finding{Check: "pii", Severity: thirdPartySeverity(thirdParty), Title: "Phone number in " + location, Description: fmt.Sprintf("A phone number pattern was detected in %s.", location), Location: sourceURL, Evidence: redactSecret(match), Remediation: "Review whether PII needs to be sent to this endpoint."}
}

func (s *Scanner) scanForPII(content, sourceURL, location string, thirdParty bool) []Finding {
	if len(content) > 10240 {
		content = content[:10240]
	}
	var findings []Finding
	for _, finding := range []*Finding{
		scanForSSN(content, sourceURL, location, thirdParty),
		scanForCreditCard(content, sourceURL, location),
		scanForEmailPII(content, sourceURL, location, thirdParty),
		scanForPhonePII(content, sourceURL, location, thirdParty),
	} {
		if finding != nil {
			findings = append(findings, *finding)
		}
	}
	return findings
}

func detectPIIFields(body string) []string {
	var fields []string
	if emailPattern.MatchString(body) {
		fields = append(fields, "email")
	}
	if phonePattern.MatchString(body) {
		fields = append(fields, "phone")
	}
	if ssnPattern.MatchString(body) {
		fields = append(fields, "ssn")
	}
	return fields
}

func looksLikeCreditCard(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum, double := 0, false
	for i := len(digits) - 1; i >= 0; i-- {
		digit := int(digits[i] - '0')
		if digit < 0 || digit > 9 {
			return false
		}
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}
