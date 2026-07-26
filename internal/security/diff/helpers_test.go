// helpers_test.go — Branch tests for cookie-flag diffing and duration formatting.
// Purpose: Coverage-expansion tests for security diff edge cases and branch paths.
// Docs: docs/features/feature/security-hardening/index.md
package diff

import (
	"testing"
	"time"
)

// ============================================
// formatDuration — All time range branches
// ============================================

func TestFormatDuration_SubSecond(t *testing.T) {
	t.Parallel()
	got := formatDuration(500 * time.Millisecond)
	if got != "0.5s" {
		t.Errorf("formatDuration(500ms) = %q, want 0.5s", got)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	t.Parallel()
	got := formatDuration(30 * time.Second)
	if got != "30s" {
		t.Errorf("formatDuration(30s) = %q, want 30s", got)
	}
}

func TestFormatDuration_MinutesOnly(t *testing.T) {
	t.Parallel()
	got := formatDuration(5 * time.Minute)
	if got != "5m" {
		t.Errorf("formatDuration(5m) = %q, want 5m", got)
	}
}

func TestFormatDuration_MinutesAndSeconds(t *testing.T) {
	t.Parallel()
	got := formatDuration(5*time.Minute + 30*time.Second)
	if got != "5m30s" {
		t.Errorf("formatDuration(5m30s) = %q, want 5m30s", got)
	}
}

func TestFormatDuration_HoursOnly(t *testing.T) {
	t.Parallel()
	got := formatDuration(2 * time.Hour)
	if got != "2h" {
		t.Errorf("formatDuration(2h) = %q, want 2h", got)
	}
}

func TestFormatDuration_HoursAndMinutes(t *testing.T) {
	t.Parallel()
	got := formatDuration(2*time.Hour + 15*time.Minute)
	if got != "2h15m" {
		t.Errorf("formatDuration(2h15m) = %q, want 2h15m", got)
	}
}

func TestFormatDuration_ExactSecondBoundary(t *testing.T) {
	t.Parallel()
	got := formatDuration(1 * time.Second)
	if got != "1s" {
		t.Errorf("formatDuration(1s) = %q, want 1s", got)
	}
}

func TestFormatDuration_JustUnderMinute(t *testing.T) {
	t.Parallel()
	got := formatDuration(59 * time.Second)
	if got != "59s" {
		t.Errorf("formatDuration(59s) = %q, want 59s", got)
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
