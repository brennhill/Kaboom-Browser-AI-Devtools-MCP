// checks_policy.go — Cookie, header, transport, authentication, and origin policy checks.
// Purpose: Owns browser transport and response-policy findings.
// Why: These checks change together when the accepted browser security posture changes.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/netflag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// ============================================
// Transport Security Check
// ============================================

func (s *Scanner) checkTransport(bodies []types.NetworkBody, pageURLs []string) []Finding {
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

func (s *Scanner) checkAuthPatterns(bodies []types.NetworkBody) []Finding {
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

func isSensitiveCookieName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"session", "token", "auth", "jwt", "sid"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func checkSingleCookie(cookie httpsec.CookieAttrs, bodyURL string, isHTTPS bool) []Finding {
	var findings []Finding
	isSensitive := isSensitiveCookieName(cookie.Name)
	if isSensitive && !cookie.HttpOnly {
		findings = append(findings, Finding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Session cookie '%s' missing HttpOnly flag", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' appears to be a session cookie but lacks the HttpOnly flag, making it accessible to JavaScript (XSS risk).", cookie.Name),
			Location:    bodyURL, Evidence: fmt.Sprintf("Set-Cookie: %s (no HttpOnly)", cookie.Name),
			Remediation: "Add the HttpOnly flag to prevent JavaScript access to this cookie.",
		})
	}
	if isHTTPS && !cookie.Secure {
		findings = append(findings, Finding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Cookie '%s' missing Secure flag on HTTPS", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' is set on an HTTPS page but lacks the Secure flag, meaning it could be sent over HTTP.", cookie.Name),
			Location:    bodyURL, Evidence: fmt.Sprintf("Set-Cookie: %s (no Secure)", cookie.Name),
			Remediation: "Add the Secure flag so the cookie is only sent over HTTPS.",
		})
	}
	if isSensitive && cookie.SameSite == "" {
		findings = append(findings, Finding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Cookie '%s' missing SameSite attribute", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' lacks a SameSite attribute, which may allow cross-site request forgery.", cookie.Name),
			Location:    bodyURL, Evidence: fmt.Sprintf("Set-Cookie: %s (no SameSite)", cookie.Name),
			Remediation: "Add SameSite=Lax or SameSite=Strict to prevent CSRF attacks.",
		})
	}
	return findings
}

func (s *Scanner) checkCookies(bodies []types.NetworkBody) []Finding {
	var findings []Finding
	for _, body := range bodies {
		if body.ResponseHeaders == nil || body.ResponseHeaders["Set-Cookie"] == "" {
			continue
		}
		for _, cookie := range httpsec.ParseCookies(body.ResponseHeaders["Set-Cookie"]) {
			findings = append(findings, checkSingleCookie(cookie, body.URL, strings.HasPrefix(body.URL, "https://"))...)
		}
	}
	return findings
}

func securityHeaders() [6]struct {
	Name     string
	Severity string
} {
	return [6]struct {
		Name     string
		Severity string
	}{
		{"Strict-Transport-Security", "high"},
		{"X-Content-Type-Options", "medium"},
		{"X-Frame-Options", "medium"},
		{"Content-Security-Policy", "medium"},
		{"Referrer-Policy", "low"},
		{"Permissions-Policy", "low"},
	}
}

func shouldSkipHSTS(headerName string, body types.NetworkBody) bool {
	return headerName == "Strict-Transport-Security" && (httpsec.IsLocalhostURL(body.URL) || !strings.HasPrefix(body.URL, "https://"))
}

func checkHeadersForOrigin(body types.NetworkBody, origin string) []Finding {
	var findings []Finding
	for _, header := range securityHeaders() {
		if shouldSkipHSTS(header.Name, body) {
			continue
		}
		if body.ResponseHeaders == nil || body.ResponseHeaders[header.Name] == "" {
			findings = append(findings, Finding{
				Check: "headers", Severity: header.Severity,
				Title:       fmt.Sprintf("Missing %s header", header.Name),
				Description: fmt.Sprintf("The response from %s does not include the %s header.", origin, header.Name),
				Location:    body.URL, Evidence: "Header not present in response",
				Remediation: fmt.Sprintf("Add the %s header to your server responses.", header.Name),
			})
		}
	}
	return findings
}

func (s *Scanner) checkSecurityHeaders(bodies []types.NetworkBody) []Finding {
	var findings []Finding
	checkedOrigins := make(map[string]bool)
	for _, body := range bodies {
		if !httpsec.IsHTMLResponse(body) {
			continue
		}
		origin := util.ExtractOrigin(body.URL)
		if checkedOrigins[origin] {
			continue
		}
		checkedOrigins[origin] = true
		findings = append(findings, checkHeadersForOrigin(body, origin)...)
	}
	return findings
}

func (s *Scanner) checkNetworkSecurity(entries []types.NetworkWaterfallEntry, pageURLs []string) []Finding {
	var findings []Finding
	pageURL := ""
	if len(pageURLs) > 0 {
		pageURL = pageURLs[0]
	}
	for _, entry := range entries {
		for _, flag := range netflag.Analyze(entry, pageURL) {
			findings = append(findings, Finding{
				Check: "network", Severity: flag.Severity, Title: flag.Message,
				Description: networkFlagDescription(flag.Type), Location: flag.Resource,
				Evidence: flag.Origin, Remediation: networkFlagRemediation(flag.Type),
			})
		}
	}
	return findings
}

func networkFlagDescription(flagType string) string {
	switch flagType {
	case "suspicious_tld":
		return "Resource loaded from a TLD with elevated abuse rates. May indicate a supply chain attack or compromised dependency."
	case "non_standard_port":
		return "Resource loaded from a non-standard port, which may indicate compromised or temporary infrastructure."
	case "mixed_content":
		return "HTTP resource loaded on an HTTPS page. An attacker on the network can modify this resource."
	case "ip_address_origin":
		return "Resource loaded from an IP address instead of a domain name. May indicate compromised or ephemeral infrastructure."
	case "potential_typosquatting":
		return "Domain is suspiciously similar to a popular CDN or service. May be a typosquatting attack."
	default:
		return "Suspicious network origin detected."
	}
}

func networkFlagRemediation(flagType string) string {
	switch flagType {
	case "suspicious_tld":
		return "Verify the domain is legitimate. Consider using well-known CDNs for third-party resources."
	case "non_standard_port":
		return "Use standard ports (80/443) for production resources. Investigate why a non-standard port is in use."
	case "mixed_content":
		return "Upgrade all resource URLs to HTTPS. Use Content-Security-Policy: upgrade-insecure-requests."
	case "ip_address_origin":
		return "Use domain names with proper DNS. Investigate why a direct IP address is being used."
	case "potential_typosquatting":
		return "Verify the exact domain name. Check package.json / CDN references for typos."
	default:
		return "Investigate the flagged origin and verify it is legitimate."
	}
}
