// Purpose: Validates HTTP methods, form action URLs, cookie headers, OS automation paths, and sanitizes inputs for safe embedding.
// Docs: docs/features/feature/file-upload/index.md

package uploadsec

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// AllowedHTTPMethods is the set of HTTP methods permitted for form submission.
var AllowedHTTPMethods = map[string]bool{
	"POST":  true,
	"PUT":   true,
	"PATCH": true,
}

// ValidateHTTPMethod checks that the method is in the allowlist.
func ValidateHTTPMethod(method string) error {
	upper := strings.ToUpper(method)
	if !AllowedHTTPMethods[upper] {
		return fmt.Errorf("HTTP method %q is not allowed. Use POST, PUT, or PATCH", method)
	}
	return nil
}

// ValidateFormActionURL validates the form submission target URL to prevent SSRF.
// Only allows http/https schemes and blocks requests to private/reserved IP ranges.
func ValidateFormActionURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	// Only allow http and https schemes
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed. Only http and https are permitted", u.Scheme)
	}

	// Resolve the hostname to check for private IPs
	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}

	// Check --ssrf-allow-host flag (test use: allows localhost test servers)
	hostPort := hostname
	if u.Port() != "" {
		hostPort = hostname + ":" + u.Port()
	}
	if IsSSRFAllowedHost(hostPort) || IsSSRFAllowedHost(hostname) {
		return nil
	}

	// Block well-known loopback/metadata hostnames
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || lowerHost == "metadata.google.internal" {
		return fmt.Errorf("hostname %q is not allowed", hostname)
	}

	// In test mode, allow private IPs (httptest.NewServer uses 127.0.0.1)
	if skipSSRFCheckEnabled() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), SSRFLookupTimeout)
	defer cancel()

	if _, err := ResolvePublicIP(ctx, hostname); err != nil {
		return err
	}

	return nil
}

// ValidateCookieHeader rejects cookie values containing header injection characters.
func ValidateCookieHeader(cookies string) error {
	if cookies == "" {
		return nil
	}
	if strings.ContainsAny(cookies, "\r\n\x00") {
		return fmt.Errorf("cookies contain invalid characters (newline or null byte)")
	}
	return nil
}

// ValidatePathForOSAutomation rejects path bytes that can break script framing
// in OS automation handlers. Shell metacharacters are allowed because callers
// use transport-specific escaping/sanitization before script execution.
func ValidatePathForOSAutomation(filePath string) error {
	// Reject null bytes (path traversal via null byte injection)
	if strings.ContainsRune(filePath, 0) {
		return fmt.Errorf("file path contains null byte")
	}
	// Reject newlines (can break AppleScript/PowerShell script structure)
	if strings.ContainsAny(filePath, "\n\r") {
		return fmt.Errorf("file path contains newline characters")
	}
	// Reject backticks (shell command substitution in PowerShell)
	if strings.Contains(filePath, "`") {
		return fmt.Errorf("file path contains backtick characters")
	}
	return nil
}

// ============================================
// Input Sanitizers
// ============================================

// SanitizeForContentDisposition removes characters that could break Content-Disposition
// header framing (quotes, newlines, null bytes). Prevents multipart header injection.
func SanitizeForContentDisposition(s string) string {
	s = strings.ReplaceAll(s, `"`, "_")
	s = strings.ReplaceAll(s, "\n", "_")
	s = strings.ReplaceAll(s, "\r", "_")
	s = strings.ReplaceAll(s, "\x00", "_")
	return s
}

// SanitizeForAppleScript escapes string delimiters for safe embedding in
// AppleScript double-quoted string literals.
func SanitizeForAppleScript(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return escaped
}

// SanitizeForSendKeys escapes a string for safe use with SendKeys.
// SendKeys treats +, ^, %, ~, (, ), {, } as special characters.
func SanitizeForSendKeys(s string) string {
	replacer := strings.NewReplacer(
		"+", "{+}",
		"^", "{^}",
		"%", "{%}",
		"~", "{~}",
		"(", "{(}",
		")", "{)}",
		"{", "{{}",
		"}", "{}}",
	)
	return replacer.Replace(s)
}
