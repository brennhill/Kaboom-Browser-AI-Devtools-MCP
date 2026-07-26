// checks_transport_auth.go — Transport encryption and unauthenticated-PII checks.
// Purpose: Validates transport encryption usage and auth-protected response patterns.
// Why: Keeps network/auth exposure heuristics separate from header/cookie/PII checks.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
)

// ============================================
// Transport Security Check
// ============================================

func (s *Scanner) checkTransport(bodies []capture.NetworkBody, pageURLs []string) []Finding {
	var findings []Finding

	pageIsHTTPS := false
	for _, pageURL := range pageURLs {
		if strings.HasPrefix(pageURL, "https://") {
			pageIsHTTPS = true
			break
		}
	}

	for _, body := range bodies {
		if !strings.HasPrefix(body.URL, "http://") {
			continue
		}
		if httpsec.IsLocalhostURL(body.URL) {
			continue
		}

		findings = append(findings, Finding{
			Check:       "transport",
			Severity:    "warning",
			Title:       "API call over unencrypted HTTP",
			Description: fmt.Sprintf("%s %s uses unencrypted HTTP. Data in transit can be intercepted.", body.Method, body.URL),
			Location:    body.URL,
			Evidence:    fmt.Sprintf("%s %s", body.Method, body.URL),
			Remediation: "Use HTTPS for all API calls. Configure your server with TLS.",
		})

		if pageIsHTTPS {
			severity := "warning"
			if httpsec.IsJavaScriptContent(body.ContentType) {
				severity = "critical"
			}
			findings = append(findings, Finding{
				Check:       "transport",
				Severity:    severity,
				Title:       "Mixed content: HTTPS page loading HTTP resource",
				Description: fmt.Sprintf("An HTTPS page is loading a resource from %s over unencrypted HTTP.", body.URL),
				Location:    body.URL,
				Evidence:    fmt.Sprintf("HTTP resource on HTTPS page: %s", body.URL),
				Remediation: "Use HTTPS for all resources. Mixed content can be intercepted by network attackers.",
			})
		}
	}

	return findings
}

// ============================================
// Auth Pattern Check
// ============================================

func (s *Scanner) checkAuthPatterns(bodies []capture.NetworkBody) []Finding {
	var findings []Finding

	for _, body := range bodies {
		if body.HasAuthHeader {
			continue
		}
		if body.ResponseBody == "" {
			continue
		}

		piiFields := detectPIIFields(body.ResponseBody)
		if len(piiFields) > 0 {
			findings = append(findings, Finding{
				Check:       "auth",
				Severity:    "warning",
				Title:       "Endpoint returns sensitive data without authentication",
				Description: fmt.Sprintf("GET %s returned PII fields (%s) but no Authorization header was present.", body.URL, strings.Join(piiFields, ", ")),
				Location:    body.URL,
				Evidence:    fmt.Sprintf("PII fields in response: %s, auth: none", strings.Join(piiFields, ", ")),
				Remediation: "Ensure this endpoint requires authentication. If public by design, verify no sensitive data is exposed.",
			})
		}
	}

	return findings
}
