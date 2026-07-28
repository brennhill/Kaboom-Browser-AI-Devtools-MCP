// checks_headers.go — Required response security header checks.
// Purpose: Evaluates required response security headers.
// Why: Keeps header-policy enforcement independent from other check categories.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// ============================================
// Security Headers Check
// ============================================

var requiredSecurityHeaders = []struct {
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

// shouldSkipHSTS returns true if an HSTS check should be skipped for this body.
func shouldSkipHSTS(headerName string, body types.NetworkBody) bool {
	if headerName != "Strict-Transport-Security" {
		return false
	}
	return httpsec.IsLocalhostURL(body.URL) || !strings.HasPrefix(body.URL, "https://")
}

// checkHeadersForOrigin checks a single HTML response for missing security headers.
func checkHeadersForOrigin(body types.NetworkBody, origin string) []Finding {
	var findings []Finding
	for _, header := range requiredSecurityHeaders {
		if shouldSkipHSTS(header.Name, body) {
			continue
		}
		if body.ResponseHeaders == nil || body.ResponseHeaders[header.Name] == "" {
			findings = append(findings, Finding{
				Check:       "headers",
				Severity:    header.Severity,
				Title:       fmt.Sprintf("Missing %s header", header.Name),
				Description: fmt.Sprintf("The response from %s does not include the %s header.", origin, header.Name),
				Location:    body.URL,
				Evidence:    "Header not present in response",
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
