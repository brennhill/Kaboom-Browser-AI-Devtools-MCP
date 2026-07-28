// middleware.go — HTTP middleware for CORS handling, extension origin validation, and request logging.
// Why: Enforces browser extension origin checks and cross-origin policy at the middleware layer before handlers run.
// Docs: docs/features/feature/security-hardening/index.md

package httpguard

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func isValidExtensionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// isExtensionOrigin checks if origin matches a browser extension scheme.
// When KABOOM_EXTENSION_ID or KABOOM_FIREFOX_EXTENSION_ID env var is set,
// only that specific ID is allowed. Otherwise any valid extension ID is accepted.
func isExtensionOrigin(origin string) (matched bool, allowed bool) {
	type extCheck struct {
		prefix string
		envVar string
	}
	checks := []extCheck{
		{"chrome-extension://", "KABOOM_EXTENSION_ID"},
		{"moz-extension://", "KABOOM_FIREFOX_EXTENSION_ID"},
	}
	for _, c := range checks {
		if strings.HasPrefix(origin, c.prefix) {
			id := strings.TrimPrefix(origin, c.prefix)
			if !isValidExtensionID(id) {
				return true, false
			}

			expectedID := strings.TrimSpace(os.Getenv(c.envVar))
			if expectedID != "" {
				return true, id == expectedID
			}

			return true, true
		}
	}
	return false, false
}

// isAllowedOrigin checks if an Origin header value is from localhost or a browser extension.
// Returns true for empty origin (CLI/curl), localhost variants, and browser extension origins.
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	if matched, allowed := isExtensionOrigin(origin); matched {
		return allowed
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	hostname := u.Hostname()
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// isAllowedHost checks if the Host header is a localhost variant.
// Returns true for empty host (HTTP/1.0 clients), localhost, 127.0.0.1, and [::1]
// with any port. This prevents DNS rebinding attacks where attacker.com resolves
// to 127.0.0.1 — the browser sends Host: attacker.com which we reject.
func isAllowedHost(host string) bool {
	if host == "" {
		return true
	}

	// Strip port if present. net.SplitHostPort fails for hosts without port,
	// so we check both forms.
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	// Strip IPv6 brackets (e.g., "[::1]" → "::1") for bare IPv6 without port
	hostname = strings.TrimPrefix(hostname, "[")
	hostname = strings.TrimSuffix(hostname, "]")

	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// CORS middleware with Host and Origin validation for DNS rebinding protection
// (MCP spec §base/transports H-2/H-3).
//
// Security: Two layers of protection against DNS rebinding:
//  1. Host header validation — rejects requests where Host is not a localhost variant.
//  2. Origin validation — rejects requests from non-local, non-extension origins.
//  3. CORS origin echo — returns the specific allowed origin, never wildcard "*".
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Layer 1: Validate Host header (DNS rebinding protection)
		if !isAllowedHost(r.Host) {
			http.Error(w, "Invalid Host header", http.StatusForbidden)
			return
		}

		// Layer 2: Validate Origin header — if present and invalid, reject with 403
		origin := r.Header.Get("Origin")
		if origin != "" && !isAllowedOrigin(origin) {
			http.Error(w, `{"error":"forbidden: invalid origin"}`, http.StatusForbidden)
			return
		}

		// Layer 3: Echo back the specific allowed origin (never wildcard "*")
		// Only set ACAO when an Origin header is present and valid.
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Kaboom-Key, X-Kaboom-Client, X-Kaboom-Extension-Version")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// ExtensionOnly wraps a handler to require the X-Kaboom-Client header
// from the Kaboom browser extension. Accepts:
//   - "kaboom-extension" (exact match)
//   - "kaboom-extension/{version}" (e.g., kaboom-extension/6.0.3)
//   - "kaboom-extension-offscreen" (offscreen recording worker)
//
// Rejects with 403 if missing or invalid. This ensures only the Kaboom
// browser extension can call extension-facing endpoints.
func ExtensionOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := r.Header.Get("X-Kaboom-Client")
		if client != "kaboom-extension" &&
			client != "kaboom-extension-offscreen" &&
			!strings.HasPrefix(client, "kaboom-extension/") {
			http.Error(w, `{"error":"forbidden: missing or invalid X-Kaboom-Client header"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
