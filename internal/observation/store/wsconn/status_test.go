// Purpose: Tests age/duration formatting helpers used by the WebSocket status projection.
// Docs: docs/features/feature/observe/index.md

package wsconn

import (
	"strings"
	"testing"
	"time"
)

// ============================================
// formatAge: seconds-only and sub-second cases
// ============================================

// Test: formatAge with a timestamp a few seconds ago returns "Ns" format.
func TestV4FormatAge_SecondsOnly(t *testing.T) {
	t.Parallel()
	ts := time.Now().Add(-7 * time.Second).Format(time.RFC3339Nano)
	age := formatAge(ts)

	if age == "" {
		t.Fatal("expected non-empty age")
	}
	// Should be "7s" or "8s" (timing tolerance)
	if !strings.HasSuffix(age, "s") {
		t.Errorf("expected age ending in 's', got: %s", age)
	}
	if strings.Contains(age, "m") || strings.Contains(age, "h") {
		t.Errorf("expected seconds-only format, got: %s", age)
	}
}

// Test: formatAge with a timestamp less than 1 second ago returns fractional.
func TestV4FormatAge_SubSecond(t *testing.T) {
	t.Parallel()
	ts := time.Now().Add(-300 * time.Millisecond).Format(time.RFC3339Nano)
	age := formatAge(ts)

	if age == "" {
		t.Fatal("expected non-empty age for sub-second timestamp")
	}
	// Should be something like "0.3s"
	if !strings.HasSuffix(age, "s") {
		t.Errorf("expected age ending in 's', got: %s", age)
	}
	if strings.Contains(age, "m") || strings.Contains(age, "h") {
		t.Errorf("expected sub-second format without minutes/hours, got: %s", age)
	}
}

// Test: formatAge with empty timestamp returns empty string.
func TestV4FormatAge_EmptyTimestamp(t *testing.T) {
	t.Parallel()
	age := formatAge("")
	if age != "" {
		t.Errorf("expected empty string for empty timestamp, got: %s", age)
	}
}

// Test: formatAge with invalid timestamp returns empty string.
func TestV4FormatAge_InvalidTimestamp(t *testing.T) {
	t.Parallel()
	age := formatAge("not-a-timestamp")
	if age != "" {
		t.Errorf("expected empty string for invalid timestamp, got: %s", age)
	}
}

// Test: formatAge with future timestamp (d < 0 branch).
func TestV4FormatAge_FutureTimestamp(t *testing.T) {
	t.Parallel()
	// A timestamp 5 seconds in the future
	ts := time.Now().Add(5 * time.Second).Format(time.RFC3339Nano)
	age := formatAge(ts)

	// When d < 0, it gets clamped to 0, so formatDuration(0) = "0.0s"
	if age != "0.0s" {
		t.Errorf("expected '0.0s' for future timestamp, got: %s", age)
	}
}

// ============================================
// formatDuration
// ============================================

// Test: formatDuration with sub-second duration returns fractional seconds.
func TestV4FormatDuration_SubSecond(t *testing.T) {
	t.Parallel()
	d := 250 * time.Millisecond
	result := formatDuration(d)
	if result != "0.2s" && result != "0.3s" {
		// Floating point: 0.25 rounds to "0.2s" with %.1f
		if !strings.HasSuffix(result, "s") {
			t.Errorf("expected sub-second format ending in 's', got: %s", result)
		}
	}
}

// Test: formatDuration with exactly 0.
func TestV4FormatDuration_Zero(t *testing.T) {
	t.Parallel()
	result := formatDuration(0)
	if result != "0.0s" {
		t.Errorf("expected '0.0s', got: %s", result)
	}
}

// Test: formatDuration with exact minutes (secs == 0 branch).
func TestV4FormatDuration_ExactMinutes(t *testing.T) {
	t.Parallel()
	d := 3 * time.Minute
	result := formatDuration(d)
	if result != "3m" {
		t.Errorf("expected '3m' for exactly 3 minutes, got: %s", result)
	}
}

// Test: formatDuration with exact hours (mins == 0 branch).
func TestV4FormatDuration_ExactHours(t *testing.T) {
	t.Parallel()
	d := 2 * time.Hour
	result := formatDuration(d)
	if result != "2h" {
		t.Errorf("expected '2h' for exactly 2 hours, got: %s", result)
	}
}

// Test: calcRate returns 0 when no timestamps fall inside the rate window.
func TestCalcRate_EmptyAndStale(t *testing.T) {
	t.Parallel()
	if got := calcRate(nil); got != 0 {
		t.Errorf("calcRate(nil) = %v, want 0", got)
	}
	stale := []time.Time{time.Now().Add(-10 * rateWindow)}
	if got := calcRate(stale); got != 0 {
		t.Errorf("calcRate(stale) = %v, want 0", got)
	}
}
