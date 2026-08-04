// tools_interact_utils_test.go — Tests for applyJitter and resolveNavigateURL.
package main

import "testing"

// ============================================
// applyJitter — read-only actions return 0
// ============================================

func TestApplyJitter_ReadOnlyActions_ReturnZero(t *testing.T) {
	t.Parallel()

	readOnlyActions := []string{
		"list_interactive",
		"get_text",
		"get_value",
		"get_attribute",
		"query",
		"list_states",
		"get_readable",
		"get_markdown",
		"explore_page",
		"run_a11y_and_export_sarif",
		"wait_for",
		"wait_for_stable",
		"auto_dismiss_overlays",
		"batch",
		"highlight",
		"subtitle",
		"clipboard_read",
	}

	for _, action := range readOnlyActions {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			h, _, _ := makeToolHandler(t)

			// Set a high jitter so we can confirm it is still skipped.
			h.interactRuntime.SetJitter(5000)

			got := h.interactRuntime.ApplyJitter(action)
			if got != 0 {
				t.Errorf("applyJitter(%q) = %d, want 0 for read-only action", action, got)
			}
		})
	}
}

// ============================================
// applyJitter — non-read-only with zero maxMs
// ============================================

func TestApplyJitter_ZeroMaxMs_ReturnsZero(t *testing.T) {
	t.Parallel()

	nonReadOnlyActions := []string{
		"click",
		"type",
		"navigate",
		"select",
		"check",
		"focus",
		"scroll_to",
		"key_press",
	}

	for _, action := range nonReadOnlyActions {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			h, _, _ := makeToolHandler(t)

			// Default actionJitterMaxMs is 0.
			got := h.interactRuntime.ApplyJitter(action)
			if got != 0 {
				t.Errorf("applyJitter(%q) = %d, want 0 when maxMs is 0", action, got)
			}
		})
	}
}

// ============================================
// applyJitter — positive maxMs returns [0, maxMs)
// ============================================

func TestApplyJitter_PositiveMaxMs_ReturnsValueInRange(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	maxMs := 2
	h.interactRuntime.SetJitter(maxMs)

	// Run multiple iterations to gain confidence the value stays in range.
	for i := 0; i < 100; i++ {
		got := h.interactRuntime.ApplyJitter("click")
		if got < 0 || got >= maxMs {
			t.Fatalf("applyJitter(\"click\") iteration %d = %d, want [0, %d)", i, got, maxMs)
		}
	}
}

// ============================================
// applyJitter — setting actionJitterMaxMs via configure
// ============================================

func TestApplyJitter_UsesConfiguredJitter(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Initially no jitter.
	if got := h.interactRuntime.ApplyJitter("click"); got != 0 {
		t.Fatalf("applyJitter before configure = %d, want 0", got)
	}

	// Set jitter via the configure path.
	resp := callConfigureRaw(h, `{"what":"action_jitter","action_jitter_ms":2}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("configure action_jitter failed: %s", firstText(result))
	}

	// Use a production-valid but tiny range so this unit contract validates the
	// configured bound without accumulating randomized wall-clock sleeps.
	for i := 0; i < 50; i++ {
		got := h.interactRuntime.ApplyJitter("click")
		if got < 0 || got >= 2 {
			t.Fatalf("applyJitter after configure iteration %d = %d, want [0, 2)", i, got)
		}
	}
}
