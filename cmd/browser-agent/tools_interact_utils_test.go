// tools_interact_utils_test.go — Tests for applyJitter and resolveNavigateURL.
package main

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

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
		"state_list",
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
			h.interactAction().SetJitter(5000)

			got := h.interactAction().ApplyJitter(action)
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
			got := h.interactAction().ApplyJitter(action)
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

	maxMs := 50
	h.interactAction().SetJitter(maxMs)

	// Run multiple iterations to gain confidence the value stays in range.
	for i := 0; i < 100; i++ {
		got := h.interactAction().ApplyJitter("click")
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
	if got := h.interactAction().ApplyJitter("click"); got != 0 {
		t.Fatalf("applyJitter before configure = %d, want 0", got)
	}

	// Set jitter via the configure path.
	resp := callConfigureRaw(h, `{"what":"action_jitter","action_jitter_ms":100}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("configure action_jitter failed: %s", firstText(result))
	}

	// Now applyJitter should return values in [0, 100).
	for i := 0; i < 50; i++ {
		got := h.interactAction().ApplyJitter("click")
		if got < 0 || got >= 100 {
			t.Fatalf("applyJitter after configure iteration %d = %d, want [0, 100)", i, got)
		}
	}
}

// ============================================
// resolveNavigateURL — normal URL passes through
// ============================================

func TestResolveNavigateURL_NormalURL_PassesThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"https URL", "https://example.com"},
		{"http URL", "http://example.com/path?q=1"},
		{"URL with fragment", "https://example.com/page#section"},
		{"URL with whitespace", "  https://example.com  "},
		{"file URL", "file:///tmp/test.html"},
		{"ftp URL", "ftp://files.example.com/doc"},
		{"chrome URL", "chrome://settings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, _, _ := makeToolHandler(t)

			got, err := h.interactAction().ResolveNavigateURLImpl(tt.url)
			if err != nil {
				t.Fatalf("resolveNavigateURL(%q) error: %v", tt.url, err)
			}
			want := strings.TrimSpace(tt.url)
			if got != want {
				t.Errorf("resolveNavigateURL(%q) = %q, want %q", tt.url, got, want)
			}
		})
	}
}

// ============================================
// resolveNavigateURL — empty URL
// ============================================

func TestResolveNavigateURL_EmptyURL_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	got, err := h.interactAction().ResolveNavigateURLImpl("")
	if err != nil {
		t.Fatalf("resolveNavigateURL(\"\") unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("resolveNavigateURL(\"\") = %q, want empty string", got)
	}
}

// ============================================
// resolveNavigateURL — kaboom-insecure:// handling
// ============================================

func TestResolveNavigateURL_KaboomInsecure_NilCapture_ReturnsError(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	h.capture = nil

	_, err := h.interactAction().ResolveNavigateURLImpl("kaboom-insecure://https://example.com")
	if err == nil {
		t.Fatal("expected error when capture is nil")
	}
	if !strings.Contains(err.Error(), "capture not initialized") {
		t.Errorf("error = %q, want to contain 'capture not initialized'", err.Error())
	}
}

func TestResolveNavigateURL_KaboomInsecure_WrongSecurityMode_ReturnsError(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	// Default security mode is "normal", not "insecure_proxy".
	_ = cap

	_, err := h.interactAction().ResolveNavigateURLImpl("kaboom-insecure://https://example.com")
	if err == nil {
		t.Fatal("expected error when security mode is not insecure_proxy")
	}
	if !strings.Contains(err.Error(), "insecure_proxy") {
		t.Errorf("error = %q, want to contain 'insecure_proxy'", err.Error())
	}
}

func TestResolveNavigateURL_KaboomInsecure_MissingTarget_ReturnsError(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	_, err := h.interactAction().ResolveNavigateURLImpl("kaboom-insecure://")
	if err == nil {
		t.Fatal("expected error for empty target URL")
	}
	if !strings.Contains(err.Error(), "target URL is empty") {
		t.Errorf("error = %q, want to contain 'target URL is empty'", err.Error())
	}
}

func TestResolveNavigateURL_KaboomInsecure_InvalidScheme_ReturnsError(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	_, err := h.interactAction().ResolveNavigateURLImpl("kaboom-insecure://ftp://files.example.com")
	if err == nil {
		t.Fatal("expected error for non-http/https target scheme")
	}
	if !strings.Contains(err.Error(), "must use http or https") {
		t.Errorf("error = %q, want to contain 'must use http or https'", err.Error())
	}
}

func TestResolveNavigateURL_KaboomInsecure_MissingHost_ReturnsError(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	_, err := h.interactAction().ResolveNavigateURLImpl("kaboom-insecure://http://")
	if err == nil {
		t.Fatal("expected error for target URL missing host")
	}
	if !strings.Contains(err.Error(), "must include host") {
		t.Errorf("error = %q, want to contain 'must include host'", err.Error())
	}
}

func TestResolveNavigateURL_KaboomInsecure_ValidTarget_ReturnsProxyURL(t *testing.T) {
	t.Parallel()
	h, server, cap := makeToolHandler(t)
	cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	port := server.getListenPort()

	tests := []struct {
		name       string
		input      string
		wantTarget string
	}{
		{
			name:       "https target",
			input:      "kaboom-insecure://https://example.com/page",
			wantTarget: "https://example.com/page",
		},
		{
			name:       "http target",
			input:      "kaboom-insecure://http://localhost:3000/api",
			wantTarget: "http://localhost:3000/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := h.interactAction().ResolveNavigateURLImpl(tt.input)
			if err != nil {
				t.Fatalf("resolveNavigateURL(%q) error: %v", tt.input, err)
			}

			// Should start with http://127.0.0.1:{port}/insecure-proxy?target=
			wantPrefix := "http://127.0.0.1:"
			if !strings.HasPrefix(got, wantPrefix) {
				t.Errorf("result = %q, want prefix %q", got, wantPrefix)
			}
			if !strings.Contains(got, "/insecure-proxy?target=") {
				t.Errorf("result = %q, want to contain '/insecure-proxy?target='", got)
			}
			// The port from the server should be in the URL.
			if port > 0 {
				portStr := strings.Split(strings.TrimPrefix(got, wantPrefix), "/")[0]
				if portStr == "" {
					t.Errorf("port not found in proxy URL: %s", got)
				}
			}
		})
	}
}

func TestResolveNavigateURL_KaboomInsecure_CaseInsensitive(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)

	// The prefix check is case-insensitive.
	got, err := h.interactAction().ResolveNavigateURLImpl("KABOOM-INSECURE://https://example.com")
	if err != nil {
		t.Fatalf("resolveNavigateURL with uppercase prefix error: %v", err)
	}
	if !strings.Contains(got, "/insecure-proxy?target=") {
		t.Errorf("uppercase prefix should be recognized, got: %s", got)
	}
}
