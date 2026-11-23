// visual.go — Renders annotation data into Playwright visual test source.
// Why: Keeps Playwright script generation isolated from JSON-RPC request orchestration.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotations

import (
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// GeneratePlaywrightFromAnnotations builds a Playwright script from annotation sessions.
func GeneratePlaywrightFromAnnotations(testName string, pages []*annotation.Session, store *annotation.Store) string {
	var b builder
	b.line("import { test, expect } from '@playwright/test';")
	b.line("")
	b.line("async function resolveAnnotationLocator(page, candidates, budgetMs = 5000) {")
	b.line("  const deadline = Date.now() + budgetMs;")
	b.line("  for (const raw of Array.isArray(candidates) ? candidates : []) {")
	b.line("    if (typeof raw !== 'string' || !raw) continue;")
	b.line("    const separator = raw.indexOf('=');")
	b.line("    const strategy = separator > 0 ? raw.slice(0, separator) : 'css';")
	b.line("    const value = separator > 0 ? raw.slice(separator + 1) : raw;")
	b.line("    const remaining = deadline - Date.now();")
	b.line("    if (remaining <= 0) break;")
	b.line("    try {")
	b.line("      let locator;")
	b.line("      switch (strategy) {")
	b.line("        case 'testid':")
	b.line("          locator = page.getByTestId(value);")
	b.line("          break;")
	b.line("        case 'label':")
	b.line("          locator = page.getByLabel(value);")
	b.line("          break;")
	b.line("        case 'placeholder':")
	b.line("          locator = page.getByPlaceholder(value);")
	b.line("          break;")
	b.line("        case 'text':")
	b.line("          locator = page.getByText(value, { exact: false });")
	b.line("          break;")
	b.line("        case 'role': {")
	b.line("          const [role, name] = value.split('|', 2);")
	b.line("          if (!role) continue;")
	b.line("          locator = name ? page.getByRole(role, { name }) : page.getByRole(role);")
	b.line("          break;")
	b.line("        }")
	b.line("        case 'css':")
	b.line("        default:")
	b.line("          locator = page.locator(value);")
	b.line("          break;")
	b.line("      }")
	b.line("      await expect(locator.first()).toBeVisible({ timeout: Math.min(remaining, 2000) });")
	b.line("      return locator.first();")
	b.line("    } catch {")
	b.line("      // Candidate failed to resolve or become visible; continue to next fallback.")
	b.line("    }")
	b.line("  }")
	b.line("  throw new Error('No annotation locator candidate resolved to a visible element');")
	b.line("}")
	b.line("")
	b.linef("test('%s', async ({ page }) => {", JsEscapeSingle(testName))

	for i, pg := range pages {
		if i > 0 {
			b.line("")
		}
		b.linef("  // --- Page: %s ---", pg.PageURL)
		b.linef("  await page.goto('%s');", JsEscapeSingle(pg.PageURL))
		b.line("")

		for j, ann := range pg.Annotations {
			b.linef("  // Annotation %d: %s", j+1, ann.Text)
			b.linef("  // Element: %s", ann.ElementSummary)

			// Try to get detail for richer selectors and a11y info.
			detail, found := store.GetDetail(ann.CorrelationID)
			if found {
				b.linef("  // Selector: %s", detail.Selector)
				if len(detail.A11yFlags) > 0 {
					for _, flag := range detail.A11yFlags {
						b.linef("  // TODO [a11y]: %s", flag)
					}
				}
			} else {
				b.line("  // (detail expired — re-run draw mode for full selectors)")
			}

			locatorVar := fmt.Sprintf("annotationLocator%d_%d", i+1, j+1)
			candidates := BuildLocatorCandidates(ann, detail)
			b.linef("  const %s = await resolveAnnotationLocator(page, %s);", locatorVar, JsStringArray(candidates))
			b.linef("  await expect(%s).toBeVisible();", locatorVar)

			b.linef("  await expect(%s).toHaveScreenshot('annotation-%d-%d.png');", locatorVar, i+1, j+1)
			b.line("")
		}
	}

	b.line("});")
	return b.string()
}

// BuildLocatorCandidates builds Playwright locator candidates from annotation data.
func BuildLocatorCandidates(ann annotation.Annotation, detail *annotation.Detail) []string {
	seen := make(map[string]struct{})
	candidates := make([]string, 0, 8)
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	if detail != nil {
		for _, candidate := range detail.SelectorCandidates {
			add(candidate)
		}
		if detail.ID != "" {
			add("css=#" + detail.ID)
		}
		if detail.Selector != "" {
			add("css=" + detail.Selector)
		}
	}

	if text := ExtractSummaryText(ann.ElementSummary); text != "" {
		add("text=" + text)
	}

	return candidates
}

// ExtractSummaryText extracts quoted text from an element summary.
func ExtractSummaryText(summary string) string {
	start := strings.Index(summary, "'")
	end := strings.LastIndex(summary, "'")
	if start >= 0 && end > start {
		return strings.TrimSpace(summary[start+1 : end])
	}
	return ""
}

// JsStringArray formats a string slice as a JavaScript array literal.
func JsStringArray(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[")
	for i, value := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("'")
		b.WriteString(JsEscapeSingle(value))
		b.WriteString("'")
	}
	b.WriteString("]")
	return b.String()
}

// JsEscapeSingle escapes a string for safe embedding inside JS single-quoted literals.
// Handles backslashes, single quotes, and newlines to prevent code injection.
func JsEscapeSingle(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}
