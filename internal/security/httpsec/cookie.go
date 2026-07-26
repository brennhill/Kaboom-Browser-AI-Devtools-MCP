// cookie.go — Set-Cookie header parsing into normalized attribute structs.
// Purpose: Parses Set-Cookie header strings into normalized attribute structs.
// Why: Shares cookie parsing across checks and diff computation without duplication.
// Docs: docs/features/feature/security-hardening/index.md

// Package httpsec holds the HTTP-level primitives — URL classification and
// Set-Cookie parsing — shared by the security scan and diff analyzers.
//
// It is a leaf package by design: it must not import any sibling package under
// internal/security, which is what keeps scan and diff free of a cycle.
package httpsec

import "strings"

// CookieAttrs represents parsed Set-Cookie attributes.
type CookieAttrs struct {
	Name     string
	HttpOnly bool
	Secure   bool
	SameSite string
}

// ParseCookies splits a newline-joined Set-Cookie header block into attributes.
func ParseCookies(setCookieHeader string) []CookieAttrs {
	var cookies []CookieAttrs

	lines := strings.Split(setCookieHeader, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cookie := ParseSingleCookie(line)
		cookies = append(cookies, cookie)
	}

	return cookies
}

// ParseSingleCookie parses one Set-Cookie value into its security attributes.
func ParseSingleCookie(raw string) CookieAttrs {
	parts := strings.Split(raw, ";")
	cookie := CookieAttrs{}

	if len(parts) > 0 {
		nameValue := strings.TrimSpace(parts[0])
		eqIdx := strings.Index(nameValue, "=")
		if eqIdx > 0 {
			cookie.Name = nameValue[:eqIdx]
		}
	}

	for _, part := range parts[1:] {
		attr := strings.TrimSpace(strings.ToLower(part))
		if attr == "httponly" {
			cookie.HttpOnly = true
		} else if attr == "secure" {
			cookie.Secure = true
		} else if strings.HasPrefix(attr, "samesite=") {
			cookie.SameSite = strings.TrimPrefix(attr, "samesite=")
		} else if attr == "samesite" {
			cookie.SameSite = "unspecified"
		}
	}

	return cookie
}
