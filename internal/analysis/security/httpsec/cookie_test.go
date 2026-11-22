// cookie_test.go — Unit tests for Set-Cookie attribute parsing.
// Docs: docs/features/feature/security-hardening/index.md
package httpsec

import "testing"

func TestParseCookies_MultipleLinesAndAttributes(t *testing.T) {
	t.Parallel()

	raw := "session_id=abc123; HttpOnly; Secure; SameSite=Strict\nprefs=dark; samesite=lax"
	cookies := ParseCookies(raw)

	if len(cookies) != 2 {
		t.Fatalf("ParseCookies len = %d, want 2", len(cookies))
	}

	if cookies[0].Name != "session_id" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != "strict" {
		t.Fatalf("first cookie parse mismatch: %+v", cookies[0])
	}
	if cookies[1].Name != "prefs" || cookies[1].SameSite != "lax" {
		t.Fatalf("second cookie parse mismatch: %+v", cookies[1])
	}
}

func TestParseCookies_SkipsBlankLines(t *testing.T) {
	t.Parallel()

	cookies := ParseCookies("\n  \nsession=abc\n")
	if len(cookies) != 1 || cookies[0].Name != "session" {
		t.Fatalf("ParseCookies blank-line handling = %+v", cookies)
	}
}

// ============================================
// ParseSingleCookie — edge cases
// ============================================

func TestParseSingleCookie_SameSiteNoValue(t *testing.T) {
	t.Parallel()
	cookie := ParseSingleCookie("name=val; SameSite")
	if cookie.SameSite != "unspecified" {
		t.Errorf("SameSite = %q, want unspecified", cookie.SameSite)
	}
}

func TestParseSingleCookie_AllFlags(t *testing.T) {
	t.Parallel()
	cookie := ParseSingleCookie("token=xyz; HttpOnly; Secure; SameSite=Strict")
	if cookie.Name != "token" {
		t.Errorf("Name = %q, want token", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Error("Secure = false, want true")
	}
	if cookie.SameSite != "strict" {
		t.Errorf("SameSite = %q, want strict", cookie.SameSite)
	}
}
