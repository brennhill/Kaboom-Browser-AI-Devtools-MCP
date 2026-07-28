// Purpose: Tests for test generation helper utilities.
// Docs: docs/features/feature/test-generation/index.md

// helpers_test.go — Unit tests for pure helper functions.
package testgen

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reproduction"
)

func TestGenerateTestFilename(t *testing.T) {
	t.Parallel()

	name := GenerateTestFilename(`Login failed: can't "submit"`, "playwright")
	if !strings.HasSuffix(name, ".spec.ts") {
		t.Fatalf("playwright filename = %q, want .spec.ts", name)
	}
	if strings.ContainsAny(name, `:'"/ \<>*?|%%`) {
		t.Fatalf("filename should be sanitized, got %q", name)
	}

	vitest := GenerateTestFilename("Short message", "vitest")
	if !strings.HasSuffix(vitest, ".test.ts") {
		t.Fatalf("vitest filename = %q, want .test.ts", vitest)
	}

	long := strings.Repeat("x", 80)
	longName := GenerateTestFilename(long, "playwright")
	stem := strings.TrimSuffix(longName, ".spec.ts")
	if len(stem) > 50 {
		t.Fatalf("sanitized long filename stem length = %d, want ≤50", len(stem))
	}

	urlLike := GenerateTestFilename(`data:text/html,%3C%21doctype%20html`, "playwright")
	if strings.ContainsAny(urlLike, `/:,%<>`) {
		t.Fatalf("URL-like input should be fully sanitized, got %q", urlLike)
	}
	stem = strings.TrimSuffix(urlLike, ".spec.ts")
	if stem == "" || stem == "-" {
		t.Fatalf("URL-like input should not produce empty stem, got %q", stem)
	}

	empty := GenerateTestFilename("", "playwright")
	if strings.TrimSuffix(empty, ".spec.ts") != "generated-test" {
		t.Fatalf("empty input should fallback to 'generated-test', got %q", empty)
	}
	whitespace := GenerateTestFilename("   ", "playwright")
	if strings.TrimSuffix(whitespace, ".spec.ts") != "generated-test" {
		t.Fatalf("whitespace input should fallback to 'generated-test', got %q", whitespace)
	}

	multi := GenerateTestFilename("a///b***c", "playwright")
	stem = strings.TrimSuffix(multi, ".spec.ts")
	if strings.Contains(stem, "--") {
		t.Fatalf("consecutive dashes should be collapsed, got %q", stem)
	}
	if strings.HasPrefix(stem, "-") || strings.HasSuffix(stem, "-") {
		t.Fatalf("stem should not have leading/trailing dashes, got %q", stem)
	}

	reserved := GenerateTestFilename("CON", "playwright")
	if strings.TrimSuffix(reserved, ".spec.ts") != "test-con" {
		t.Fatalf("reserved filename should be rewritten, got %q", reserved)
	}
	reserved = GenerateTestFilename("lpt1", "vitest")
	if strings.TrimSuffix(reserved, ".test.ts") != "test-lpt1" {
		t.Fatalf("reserved filename should be rewritten for vitest, got %q", reserved)
	}
}

func TestExtractSelectorsFromActions(t *testing.T) {
	t.Parallel()

	actions := []types.EnhancedAction{
		{
			Type: "click",
			Selectors: map[string]any{
				"testId": "submit-btn",
				"role":   map[string]any{"role": "button"},
				"id":     "submit",
			},
		},
		{
			Type: "click",
			Selectors: map[string]any{
				"testId": "submit-btn", // duplicate
			},
		},
		{
			Type:      "click",
			Selectors: nil,
		},
	}

	selectors := ExtractSelectorsFromActions(actions)
	if len(selectors) != 3 {
		t.Fatalf("ExtractSelectorsFromActions len = %d, want 3 unique selectors; got %+v", len(selectors), selectors)
	}

	joined := strings.Join(selectors, "\n")
	for _, want := range []string{
		`[data-testid="submit-btn"]`,
		`[role="button"]`,
		`#submit`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("selectors missing %q: %+v", want, selectors)
		}
	}
}

func TestNormalizeTimestamp(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 11, 8, 40, 0, 0, time.UTC)
	got := NormalizeTimestamp(ts.Format(time.RFC3339))
	if got != ts.UnixMilli() {
		t.Fatalf("NormalizeTimestamp(RFC3339) = %d, want %d", got, ts.UnixMilli())
	}

	if bad := NormalizeTimestamp("not-a-timestamp"); bad != 0 {
		t.Fatalf("NormalizeTimestamp(invalid) = %d, want 0", bad)
	}
}

func TestGeneratePlaywrightScript(t *testing.T) {
	t.Parallel()

	actions := []types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"id": "login"}},
		{Type: "input", Selectors: map[string]any{"id": "email"}, Value: "user@example.com"},
		{Type: "navigate", ToURL: "https://app.example.com/dashboard"},
	}

	script := reproduction.GeneratePlaywrightScript(actions, reproduction.Params{
		ErrorMessage: "Cannot read property",
		BaseURL:      "https://app.example.com",
	})

	for _, want := range []string{
		"import { test, expect } from '@playwright/test';",
		"locator('#login').click()",
		"locator('#email').fill('user@example.com')",
		"await page.goto('https://app.example.com/dashboard');",
		"// Error: Cannot read property",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("generated script missing %q\nscript:\n%s", want, script)
		}
	}
}

func TestDeriveInteractionTestName(t *testing.T) {
	t.Parallel()

	if name := DeriveInteractionTestName(nil); name != "user-interaction" {
		t.Fatalf("DeriveInteractionTestName(nil) = %q, want user-interaction", name)
	}
	if name := DeriveInteractionTestName([]types.EnhancedAction{}); name != "user-interaction" {
		t.Fatalf("DeriveInteractionTestName([]) = %q, want user-interaction", name)
	}

	actions := []types.EnhancedAction{{Type: "click", URL: "https://app.example.com/login"}}
	if name := DeriveInteractionTestName(actions); name != "https://app.example.com/login" {
		t.Fatalf("DeriveInteractionTestName(URL) = %q, want URL", name)
	}

	actions = []types.EnhancedAction{{Type: "click", URL: ""}}
	if name := DeriveInteractionTestName(actions); name != "click-flow" {
		t.Fatalf("DeriveInteractionTestName(Type) = %q, want click-flow", name)
	}

	actions = []types.EnhancedAction{{URL: "", Type: ""}}
	if name := DeriveInteractionTestName(actions); name != "user-interaction" {
		t.Fatalf("DeriveInteractionTestName(empty) = %q, want user-interaction", name)
	}

	actions = []types.EnhancedAction{{Type: "navigate", URL: "https://example.com"}}
	if name := DeriveInteractionTestName(actions); name != "https://example.com" {
		t.Fatalf("URL should take precedence over type; got %q", name)
	}
}

func TestBuildRegressionAssertions_NoErrorsNoNetwork(t *testing.T) {
	t.Parallel()

	assertions, count := BuildRegressionAssertions(nil, nil)
	if count != 1 {
		t.Fatalf("assertionCount = %d, want 1", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Assert no console errors") {
		t.Fatal("expected clean baseline assertion comment")
	}
	if !strings.Contains(joined, "expect(consoleErrors).toHaveLength(0)") {
		t.Fatal("expected consoleErrors assertion")
	}
}

func TestBuildRegressionAssertions_WithErrors(t *testing.T) {
	t.Parallel()

	errs := []string{"TypeError: undefined", "ReferenceError: foo"}
	assertions, count := BuildRegressionAssertions(errs, nil)
	if count != 0 {
		t.Fatalf("assertionCount = %d, want 0", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Baseline had 2 console errors") {
		t.Fatalf("expected baseline error count comment; got:\n%s", joined)
	}
}

func TestBuildRegressionAssertions_WithNetworkBodies(t *testing.T) {
	t.Parallel()

	bodies := []types.NetworkBody{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "POST", URL: "/api/login", Status: 201},
		{Method: "PUT", URL: "/api/update", Status: 204},
	}
	_, count := BuildRegressionAssertions(nil, bodies)
	if count != 4 {
		t.Fatalf("assertionCount = %d, want 4", count)
	}
}

func TestBuildRegressionAssertions_NetworkLimitedToThree(t *testing.T) {
	t.Parallel()

	bodies := []types.NetworkBody{
		{Method: "GET", URL: "/api/1", Status: 200},
		{Method: "GET", URL: "/api/2", Status: 200},
		{Method: "GET", URL: "/api/3", Status: 200},
		{Method: "GET", URL: "/api/4", Status: 200},
		{Method: "GET", URL: "/api/5", Status: 200},
	}
	assertions, count := BuildRegressionAssertions(nil, bodies)
	if count != 4 {
		t.Fatalf("assertionCount = %d, want 4 (max 3 network)", count)
	}
	joined := strings.Join(assertions, "\n")
	if strings.Contains(joined, "/api/4") {
		t.Fatal("should not include 4th network body")
	}
}

func TestBuildRegressionAssertions_SkipsZeroStatusNetworkBodies(t *testing.T) {
	t.Parallel()

	bodies := []types.NetworkBody{
		{Method: "GET", URL: "/api/ok", Status: 200},
		{Method: "GET", URL: "/api/zero", Status: 0},
	}
	_, count := BuildRegressionAssertions(nil, bodies)
	if count != 2 {
		t.Fatalf("assertionCount = %d, want 2 (skip status 0)", count)
	}
}

func TestInsertAssertionsBeforeClose_Normal(t *testing.T) {
	t.Parallel()

	script := "test('example', async ({ page }) => {\n  await page.click('#btn');\n});\n"
	assertions := []string{"  expect(page).toBeTruthy();", "  expect(errors).toHaveLength(0);"}

	result := InsertAssertionsBeforeClose(script, assertions)
	if !strings.Contains(result, "expect(page).toBeTruthy();") {
		t.Fatal("assertion not inserted")
	}
	assertionIdx := strings.Index(result, "expect(page)")
	closeIdx := strings.LastIndex(result, "});")
	if assertionIdx > closeIdx {
		t.Fatal("assertions should be before closing });")
	}
}

func TestInsertAssertionsBeforeClose_NoClosingBrace(t *testing.T) {
	t.Parallel()

	script := "incomplete script without closing"
	result := InsertAssertionsBeforeClose(script, []string{"  expect(1).toBe(1);"})
	if result != script {
		t.Fatalf("expected unchanged script when no }); found")
	}
}
