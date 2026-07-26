// url.go — URL classification helpers (localhost, HTML, JavaScript, third-party).
// Purpose: Provides URL parsing, localhost detection, and origin extraction utilities for security checks.
// Why: Centralizes URL helper functions used across multiple security scanning modules.
package httpsec

import (
	"net/url"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// IsLocalhostURL reports whether rawURL points at a loopback or any-address host.
func IsLocalhostURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

// IsHTMLResponse reports whether a captured body carries an HTML content type.
func IsHTMLResponse(body capture.NetworkBody) bool {
	ct := strings.ToLower(body.ContentType)
	return strings.Contains(ct, "text/html")
}

// IsJavaScriptContent reports whether a content type describes JavaScript.
func IsJavaScriptContent(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "javascript")
}

// IsThirdPartyURL reports whether requestURL falls outside the domains of every
// observed page URL. With no page URLs it conservatively returns false.
func IsThirdPartyURL(requestURL string, pageURLs []string) bool {
	if len(pageURLs) == 0 {
		return false
	}
	reqParsed, err := url.Parse(requestURL)
	if err != nil {
		return false
	}
	reqHost := reqParsed.Hostname()

	for _, pageURL := range pageURLs {
		pageParsed, err := url.Parse(pageURL)
		if err != nil {
			continue
		}
		pageHost := pageParsed.Hostname()
		// Same domain check (including subdomains)
		if reqHost == pageHost || strings.HasSuffix(reqHost, "."+pageHost) || strings.HasSuffix(pageHost, "."+reqHost) {
			return false
		}
	}
	return true
}
