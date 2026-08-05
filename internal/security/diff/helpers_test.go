// helpers_test.go — Branch tests for cookie-flag diffing and package boundaries.
// Purpose: Security diff edge cases plus its physical module boundary.
// Docs: docs/features/feature/security-hardening/index.md
package diff

import (
	"os"
	"testing"
)

func TestSecurityDiffPackageRespectsTenFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("internal/security/diff has %d files; want at most 10 change-coupled owners", files)
	}
}

// ============================================
// diffSingleCookieFlag — All branches
// ============================================

func TestDiffSingleCookieFlag_FlagRemoved(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "HttpOnly",
		fromActive: true,
		toActive:   false,
		lostMsg:    "lost HttpOnly",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change for flag removal")
	}
	if change.Change != "flag_removed" {
		t.Errorf("change = %q, want flag_removed", change.Change)
	}
	if change.Before != "present" {
		t.Errorf("before = %q, want present", change.Before)
	}
	if change.After != "absent" {
		t.Errorf("after = %q, want absent", change.After)
	}
}

func TestDiffSingleCookieFlag_FlagAdded(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "Secure",
		fromActive: false,
		toActive:   true,
		gainedMsg:  "gained Secure",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change for flag addition")
	}
	if change.Change != "flag_added" {
		t.Errorf("change = %q, want flag_added", change.Change)
	}
	if change.After != "present" {
		t.Errorf("after = %q, want present", change.After)
	}
}

func TestDiffSingleCookieFlag_SameSiteRemoved(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "SameSite",
		fromActive: true,
		toActive:   false,
		fromVal:    "Lax",
		lostMsg:    "lost SameSite",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.Before != "Lax" {
		t.Errorf("before = %q, want Lax (SameSite uses actual value)", change.Before)
	}
}

func TestDiffSingleCookieFlag_SameSiteAdded(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "SameSite",
		fromActive: false,
		toActive:   true,
		toVal:      "Strict",
		gainedMsg:  "gained SameSite",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.After != "Strict" {
		t.Errorf("after = %q, want Strict (SameSite uses actual value)", change.After)
	}
}

func TestDiffSingleCookieFlag_NoChange(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "HttpOnly",
		fromActive: true,
		toActive:   true,
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change != nil {
		t.Error("expected nil change when flag unchanged")
	}
}

// ============================================
// flagAbsentValue
// ============================================

func TestFlagAbsentValue(t *testing.T) {
	t.Parallel()
	if got := flagAbsentValue("SameSite", ""); got != "" {
		t.Errorf("flagAbsentValue(SameSite, '') = %q, want empty", got)
	}
	if got := flagAbsentValue("HttpOnly", ""); got != "absent" {
		t.Errorf("flagAbsentValue(HttpOnly, '') = %q, want absent", got)
	}
	if got := flagAbsentValue("Secure", "fallback"); got != "absent" {
		t.Errorf("flagAbsentValue(Secure, 'fallback') = %q, want absent", got)
	}
}
