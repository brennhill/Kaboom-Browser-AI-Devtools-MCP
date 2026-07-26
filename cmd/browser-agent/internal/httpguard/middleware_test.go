// Purpose: Tests for HTTP middleware chain execution.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// server_middleware_test.go — Unit tests for HTTP middleware functions.
package httpguard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAllowedOrigin(t *testing.T) {
	t.Setenv("KABOOM_EXTENSION_ID", "")
	t.Setenv("KABOOM_FIREFOX_EXTENSION_ID", "")

	tests := []struct {
		origin string
		want   bool
	}{
		// Empty origin (CLI/curl)
		{"", true},

		// Localhost variants
		{"http://localhost", true},
		{"http://localhost:3000", true},
		{"https://localhost:8443", true},
		{"http://127.0.0.1", true},
		{"http://127.0.0.1:9222", true},
		{"http://[::1]", true},
		{"http://[::1]:3000", true},

		// Browser extensions (any valid ID accepted)
		{"chrome-extension://abcdefghijklmnop", true},
		{"moz-extension://abcdefghijklmnop", true},

		// Invalid origins
		{"http://evil.com", false},
		{"https://attacker.com:3000", false},
		{"http://192.168.1.1", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		got := isAllowedOrigin(tt.origin)
		if got != tt.want {
			t.Errorf("isAllowedOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestIsAllowedHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"", true},
		{"localhost", true},
		{"localhost:3000", true},
		{"127.0.0.1", true},
		{"127.0.0.1:9222", true},
		{"[::1]", true},
		{"[::1]:3000", true},
		{"::1", true},

		// DNS rebinding attacks
		{"evil.com", false},
		{"attacker.com:3000", false},
		{"192.168.1.1", false},
	}

	for _, tt := range tests {
		got := isAllowedHost(tt.host)
		if got != tt.want {
			t.Errorf("isAllowedHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestCorsMiddleware(t *testing.T) {
	t.Setenv("KABOOM_EXTENSION_ID", "")
	t.Setenv("KABOOM_FIREFOX_EXTENSION_ID", "")

	handler := CORS(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("preflight OPTIONS", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/mcp", nil)
		req.Host = "localhost:3000"
		req.Header.Set("Origin", "http://localhost:3000")
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("OPTIONS status = %d, want 204", rr.Code)
		}
		if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatalf("ACAO = %q, want origin echo", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("rejects invalid host", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Host = "evil.com"
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("invalid host status = %d, want 403", rr.Code)
		}
	})

	t.Run("rejects invalid origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Origin", "http://evil.com")
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("invalid origin status = %d, want 403", rr.Code)
		}
	})

	t.Run("allows localhost", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Host = "localhost:3000"
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("localhost status = %d, want 200", rr.Code)
		}
	})
}

func TestExtensionOnly(t *testing.T) {
	handler := ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("rejects missing header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/data", nil)
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("missing header status = %d, want 403", rr.Code)
		}
	})

	t.Run("accepts exact match", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/data", nil)
		req.Header.Set("X-Kaboom-Client", "kaboom-extension")
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("exact match status = %d, want 200", rr.Code)
		}
	})

	t.Run("accepts versioned format", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/data", nil)
		req.Header.Set("X-Kaboom-Client", "kaboom-extension/6.0.3")
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("versioned format status = %d, want 200", rr.Code)
		}
	})

	t.Run("rejects invalid client", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/data", nil)
		req.Header.Set("X-Kaboom-Client", "other-client")
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("invalid client status = %d, want 403", rr.Code)
		}
	})
}

func TestIsAllowedOriginEnvOverrideRequiresExactExtensionID(t *testing.T) {
	t.Setenv("KABOOM_EXTENSION_ID", "allowedextensionid123")
	t.Setenv("KABOOM_FIREFOX_EXTENSION_ID", "")

	allowed := "chrome-extension://allowedextensionid123"
	rejected := "chrome-extension://differentextensionid"

	if !isAllowedOrigin(allowed) {
		t.Fatalf("configured extension origin should be allowed")
	}
	if isAllowedOrigin(rejected) {
		t.Fatalf("non-configured extension origin should be rejected")
	}
}
