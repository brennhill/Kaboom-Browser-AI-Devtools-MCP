// checks_pii.go — PII pattern detection in request/response payloads.
// Purpose: Detects PII patterns in request/response payloads.
// Why: Isolates PII heuristics and evidence shaping from other security checks.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
)

// ============================================
// PII Detection
// ============================================

var (
	emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phonePattern = regexp.MustCompile(`(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
	ssnPattern   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ccPattern    = regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`)
)

func (s *Scanner) checkPII(bodies []capture.NetworkBody, pageURLs []string) []Finding {
	var findings []Finding

	for _, body := range bodies {
		if body.RequestBody != "" {
			isThirdParty := httpsec.IsThirdPartyURL(body.URL, pageURLs)
			findings = append(findings, s.scanForPII(body.RequestBody, body.URL, "request body", isThirdParty)...)
		}

		if body.ResponseBody != "" {
			findings = append(findings, s.scanForPII(body.ResponseBody, body.URL, "response body", false)...)
		}
	}

	return findings
}

// scanForSSN checks for Social Security Number patterns.
func scanForSSN(content, sourceURL, location string, isThirdParty bool) *Finding {
	if !ssnPattern.MatchString(content) {
		return nil
	}
	match := ssnPattern.FindString(content)
	severity := "high"
	desc := fmt.Sprintf("A Social Security Number pattern was detected in %s.", location)
	if isThirdParty {
		severity = "critical"
		desc = fmt.Sprintf("A Social Security Number pattern is being sent to a third-party endpoint in %s.", location)
	}
	return &Finding{
		Check: "pii", Severity: severity,
		Title: "SSN pattern detected in " + location, Description: desc,
		Location: sourceURL, Evidence: redactSecret(match),
		Remediation: "Never transmit SSNs in plain text. Use tokenization or encryption.",
	}
}

// scanForCreditCard checks for credit card number patterns.
func scanForCreditCard(content, sourceURL, location string) *Finding {
	if !ccPattern.MatchString(content) {
		return nil
	}
	match := ccPattern.FindString(content)
	cleaned := strings.ReplaceAll(strings.ReplaceAll(match, " ", ""), "-", "")
	if len(cleaned) < 13 || len(cleaned) > 19 || !looksLikeCreditCard(cleaned) {
		return nil
	}
	return &Finding{
		Check: "pii", Severity: "critical",
		Title:       "Credit card number detected in " + location,
		Description: fmt.Sprintf("A credit card number pattern was detected in %s.", location),
		Location:    sourceURL, Evidence: redactSecret(match),
		Remediation: "Never transmit full card numbers. Use tokenization (e.g., Stripe tokens).",
	}
}

// thirdPartySeverity returns "warning" for third-party, "info" otherwise.
func thirdPartySeverity(isThirdParty bool) string {
	if isThirdParty {
		return "warning"
	}
	return "info"
}

// scanForEmailPII checks for email address patterns.
func scanForEmailPII(content, sourceURL, location string, isThirdParty bool) *Finding {
	if !emailPattern.MatchString(content) {
		return nil
	}
	return &Finding{
		Check: "pii", Severity: thirdPartySeverity(isThirdParty),
		Title:       "Email address in " + location,
		Description: fmt.Sprintf("An email address was detected in %s.", location),
		Location:    sourceURL, Evidence: redactSecret(emailPattern.FindString(content)),
		Remediation: "Review whether PII needs to be sent to this endpoint.",
	}
}

// scanForPhonePII checks for phone number patterns.
func scanForPhonePII(content, sourceURL, location string, isThirdParty bool) *Finding {
	if !phonePattern.MatchString(content) {
		return nil
	}
	match := phonePattern.FindString(content)
	cleaned := strings.NewReplacer("-", "", " ", "", "(", "").Replace(match)
	if len(cleaned) < 10 {
		return nil
	}
	return &Finding{
		Check: "pii", Severity: thirdPartySeverity(isThirdParty),
		Title:       "Phone number in " + location,
		Description: fmt.Sprintf("A phone number pattern was detected in %s.", location),
		Location:    sourceURL, Evidence: redactSecret(match),
		Remediation: "Review whether PII needs to be sent to this endpoint.",
	}
}

func (s *Scanner) scanForPII(content, sourceURL, location string, isThirdParty bool) []Finding {
	if len(content) > 10240 {
		content = content[:10240]
	}

	var findings []Finding
	piiChecks := []*Finding{
		scanForSSN(content, sourceURL, location, isThirdParty),
		scanForCreditCard(content, sourceURL, location),
		scanForEmailPII(content, sourceURL, location, isThirdParty),
		scanForPhonePII(content, sourceURL, location, isThirdParty),
	}
	for _, finding := range piiChecks {
		if finding != nil {
			findings = append(findings, *finding)
		}
	}
	return findings
}

// detectPIIFields checks a body string for PII patterns and returns field names found.
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

// looksLikeCreditCard performs a basic Luhn check on a digit string.
func looksLikeCreditCard(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
