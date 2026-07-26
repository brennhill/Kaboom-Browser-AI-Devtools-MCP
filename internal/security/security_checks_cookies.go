// Purpose: Validates cookie security attributes for session/sensitive cookies.
// Why: Isolates cookie policy logic and findings from unrelated check categories.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
)

// ============================================
// Cookie Security Check
// ============================================

var sessionCookiePattern = regexp.MustCompile(`(?i)(session|token|auth|jwt|sid)`)

// checkSingleCookie checks a single cookie for missing security attributes.
func checkSingleCookie(cookie httpsec.CookieAttrs, bodyURL string, isHTTPS bool) []SecurityFinding {
	var findings []SecurityFinding
	isSensitive := sessionCookiePattern.MatchString(cookie.Name)

	if isSensitive && !cookie.HttpOnly {
		findings = append(findings, SecurityFinding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Session cookie '%s' missing HttpOnly flag", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' appears to be a session cookie but lacks the HttpOnly flag, making it accessible to JavaScript (XSS risk).", cookie.Name),
			Location:    bodyURL,
			Evidence:    fmt.Sprintf("Set-Cookie: %s (no HttpOnly)", cookie.Name),
			Remediation: "Add the HttpOnly flag to prevent JavaScript access to this cookie.",
		})
	}
	if isHTTPS && !cookie.Secure {
		findings = append(findings, SecurityFinding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Cookie '%s' missing Secure flag on HTTPS", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' is set on an HTTPS page but lacks the Secure flag, meaning it could be sent over HTTP.", cookie.Name),
			Location:    bodyURL,
			Evidence:    fmt.Sprintf("Set-Cookie: %s (no Secure)", cookie.Name),
			Remediation: "Add the Secure flag so the cookie is only sent over HTTPS.",
		})
	}
	if isSensitive && cookie.SameSite == "" {
		findings = append(findings, SecurityFinding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Cookie '%s' missing SameSite attribute", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' lacks a SameSite attribute, which may allow cross-site request forgery.", cookie.Name),
			Location:    bodyURL,
			Evidence:    fmt.Sprintf("Set-Cookie: %s (no SameSite)", cookie.Name),
			Remediation: "Add SameSite=Lax or SameSite=Strict to prevent CSRF attacks.",
		})
	}
	return findings
}

func (s *SecurityScanner) checkCookies(bodies []capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding
	for _, body := range bodies {
		if body.ResponseHeaders == nil {
			continue
		}
		setCookie := body.ResponseHeaders["Set-Cookie"]
		if setCookie == "" {
			continue
		}
		isHTTPS := strings.HasPrefix(body.URL, "https://")
		for _, cookie := range httpsec.ParseCookies(setCookie) {
			findings = append(findings, checkSingleCookie(cookie, body.URL, isHTTPS)...)
		}
	}
	return findings
}
