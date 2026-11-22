// apikey.go — Optional API-key HTTP middleware for daemon endpoint authentication.
// Why: Provides lightweight access control for local/remote deployments without changing handler code.
// Docs: docs/features/feature/api-key-auth/index.md

package httpguard

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// APIKey returns a middleware function that checks the X-Kaboom-Key header.
// If expectedKey is empty, no authentication is required (pass-through).
// If expectedKey is set, requests must include the correct key or receive 401.
// Key comparison uses crypto/subtle.ConstantTimeCompare to prevent timing attacks.
func APIKey(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no key is configured, auth is disabled
			if expectedKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check the X-Kaboom-Key header
			providedKey := r.Header.Get("X-Kaboom-Key")

			// Use constant-time comparison to prevent timing attacks
			expectedBytes := []byte(expectedKey)
			providedBytes := []byte(providedKey)

			if subtle.ConstantTimeCompare(expectedBytes, providedBytes) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}) // #nosec G104 -- best-effort error response
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
