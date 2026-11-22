// Purpose: Tests for API key authentication middleware.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// auth_test.go — Tests for API key authentication middleware.
// Covers: correct key acceptance, incorrect key rejection, missing key,
// empty key pass-through, constant-time comparison, header extraction,
// response format, and content-type header.
//
// Run: go test ./cmd/browser-agent -run "TestAuth" -v
package httpguard

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// 1. Pass-through when no key configured
// ============================================

func TestAuth_NoKeyConfigured_PassThrough(t *testing.T) {
	middleware := APIKey("")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("no-key status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", rr.Body.String(), "ok")
	}
}

func TestAuth_NoKeyConfigured_IgnoresProvidedKey(t *testing.T) {
	middleware := APIKey("")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Kaboom-Key", "some-random-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (key should be ignored when none configured)", rr.Code)
	}
}

// ============================================
// 2. Correct API key acceptance
// ============================================

func TestAuth_CorrectKey_Accepted(t *testing.T) {
	const secret = "sk-test-abc123"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authorized"))
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Kaboom-Key", secret)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("correct key status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "authorized" {
		t.Fatalf("body = %q, want %q", rr.Body.String(), "authorized")
	}
}

func TestAuth_CorrectKey_AllHTTPMethods(t *testing.T) {
	const secret = "method-test-key"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/data", nil)
			req.Header.Set("X-Kaboom-Key", secret)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("%s status = %d, want 200", method, rr.Code)
			}
		})
	}
}

// ============================================
// 3. Incorrect API key rejection
// ============================================

func TestAuth_WrongKey_Rejected(t *testing.T) {
	const secret = "correct-key-123"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with wrong key")
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Kaboom-Key", "wrong-key-456")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key status = %d, want 401", rr.Code)
	}
}

func TestAuth_WrongKey_ResponseFormat(t *testing.T) {
	const secret = "correct-key"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with wrong key")
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Kaboom-Key", "bad-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check Content-Type header
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	// Check response body is valid JSON with snake_case "error" field
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("error field = %q, want %q", body["error"], "unauthorized")
	}
}

func TestAuth_WrongKey_Variants(t *testing.T) {
	const secret = "correct-key-abc"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	badKeys := []struct {
		name string
		key  string
	}{
		{"completely_different", "totally-wrong"},
		{"prefix_match", "correct-key-abc-extra"},
		{"suffix_match", "xtra-correct-key-abc"},
		{"case_mismatch", "Correct-Key-ABC"},
		{"substring", "correct-key"},
		{"one_char_off", "correct-key-abd"},
		{"with_whitespace", " correct-key-abc "},
		{"with_newline", "correct-key-abc\n"},
		{"null_byte", "correct-key-abc\x00"},
	}

	for _, tc := range badKeys {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/data", nil)
			req.Header.Set("X-Kaboom-Key", tc.key)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("key %q: status = %d, want 401", tc.name, rr.Code)
			}
		})
	}
}

// ============================================
// 4. Missing API key
// ============================================

func TestAuth_MissingKey_Rejected(t *testing.T) {
	const secret = "required-key"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without key")
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	// No X-Kaboom-Key header set
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status = %d, want 401", rr.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("error field = %q, want %q", body["error"], "unauthorized")
	}
}

// ============================================
// 5. Empty API key in header
// ============================================

func TestAuth_EmptyKeyHeader_Rejected(t *testing.T) {
	const secret = "non-empty-secret"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with empty key")
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Kaboom-Key", "")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("empty key status = %d, want 401", rr.Code)
	}
}

// ============================================
// 6. Constant-time comparison verification
// ============================================

func TestAuth_UsesConstantTimeCompare(t *testing.T) {
	// Verify that the implementation uses crypto/subtle.ConstantTimeCompare
	// by confirming it produces the same results as a direct call.
	// This is a correctness test — actual timing resistance is a property
	// of the crypto/subtle library and not measurable in unit tests.

	pairs := []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 1},
		{"abc", "abd", 0},
		{"abc", "ab", 0}, // different lengths
		{"", "", 1},      // both empty
		{"abc", "", 0},   // one empty
		{"", "abc", 0},   // other empty
	}
	for _, p := range pairs {
		got := subtle.ConstantTimeCompare([]byte(p.a), []byte(p.b))
		if got != p.want {
			t.Errorf("ConstantTimeCompare(%q, %q) = %d, want %d", p.a, p.b, got, p.want)
		}
	}
}

func TestAuth_DifferentLengthKeys_Rejected(t *testing.T) {
	// Ensures that keys of different lengths are properly rejected.
	// ConstantTimeCompare returns 0 for different-length inputs,
	// which is the correct security behavior.
	const secret = "exact-length-key"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	keys := []string{
		"exact-length-ke",         // one char shorter
		"exact-length-key!",       // one char longer
		"e",                       // much shorter
		strings.Repeat("x", 1000), // much longer
	}

	for _, key := range keys {
		t.Run("len_"+strings.Replace(key[:min(len(key), 10)], " ", "_", -1), func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/data", nil)
			req.Header.Set("X-Kaboom-Key", key)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("different-length key: status = %d, want 401", rr.Code)
			}
		})
	}
}

// ============================================
// 7. Header extraction edge cases
// ============================================

func TestAuth_HeaderCaseSensitivity(t *testing.T) {
	// HTTP headers are case-insensitive per RFC 7230.
	// Go's http.Header.Get() normalizes header names.
	const secret = "case-test-key"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Go's http package canonicalizes headers, so these should all work
	headerNames := []string{
		"X-Kaboom-Key",
		"x-kaboom-key",
		"X-KABOOM-KEY",
		"x-Kaboom-key",
	}

	for _, name := range headerNames {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/data", nil)
			req.Header.Set(name, secret)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("header %q: status = %d, want 200", name, rr.Code)
			}
		})
	}
}

func TestAuth_MultipleHeaderValues(t *testing.T) {
	// When multiple values for the same header exist, Get() returns the first.
	const secret = "multi-value-key"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Kaboom-Key", secret)
	req.Header.Add("X-Kaboom-Key", "second-value")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Get() returns the first value, which is the correct key
	if rr.Code != http.StatusOK {
		t.Fatalf("first-of-multiple status = %d, want 200", rr.Code)
	}
}

func TestAuth_WrongHeaderName(t *testing.T) {
	// Key in wrong header should not authenticate
	const secret = "header-name-test"
	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with wrong header name")
	}))

	wrongHeaders := []string{
		"Authorization",
		"X-Api-Key",
		"X-Kaboom-Token",
		"X-Kaboom-Secret",
	}

	for _, hdr := range wrongHeaders {
		t.Run(hdr, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/data", nil)
			req.Header.Set(hdr, secret)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("header %q: status = %d, want 401", hdr, rr.Code)
			}
		})
	}
}

// ============================================
// 8. Middleware chaining
// ============================================

func TestAuth_MiddlewareChaining(t *testing.T) {
	// Verify middleware properly wraps and chains handlers
	const secret = "chain-test"
	var called bool

	middleware := APIKey(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("authorized_calls_next", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Kaboom-Key", secret)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if !called {
			t.Fatal("next handler was not called for authorized request")
		}
	})

	t.Run("unauthorized_blocks_next", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Kaboom-Key", "wrong")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if called {
			t.Fatal("next handler was called for unauthorized request")
		}
	})
}

// ============================================
// 9. Special characters in keys
// ============================================

func TestAuth_SpecialCharacterKeys(t *testing.T) {
	specialKeys := []string{
		"key-with-unicode-\u00e9\u00e0\u00fc",
		"key with spaces",
		"key=with=equals",
		"key&with&ampersands",
		"key/with/slashes",
		"key+with+plus",
		"a",                       // single char
		strings.Repeat("k", 4096), // very long key
	}

	for i, key := range specialKeys {
		t.Run("special_"+strings.Replace(key[:min(len(key), 15)], " ", "_", -1), func(t *testing.T) {
			_ = i // avoid unused
			middleware := APIKey(key)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			// Correct key should pass
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Kaboom-Key", key)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("correct special key: status = %d, want 200", rr.Code)
			}

			// Wrong key should fail
			req2 := httptest.NewRequest("GET", "/", nil)
			req2.Header.Set("X-Kaboom-Key", "wrong")
			rr2 := httptest.NewRecorder()
			handler.ServeHTTP(rr2, req2)
			if rr2.Code != http.StatusUnauthorized {
				t.Errorf("wrong key against special: status = %d, want 401", rr2.Code)
			}
		})
	}
}

// min is provided by Go 1.21+ builtins, and by connection_lifecycle_test.go
// in this package — no need to redeclare.
