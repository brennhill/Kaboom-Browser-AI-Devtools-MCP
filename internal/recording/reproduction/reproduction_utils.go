// Purpose: Shared reproduction helpers — JS escaping, URL rewriting, and the optional extras
// (screenshots, visual assertions, network fixtures) used by both output formats.
// Why: generate(reproduction) advertises include_screenshots, visual_assertions and
// generate_fixtures; one place implements them for the Kaboom and Playwright generators.
package reproduction

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
	"net/url"
	"strings"
)

// EscapeJS escapes a string for embedding in JavaScript string literals.
func EscapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// rewriteURL replaces the origin of a URL with baseURL.
func rewriteURL(originalURL, baseURL string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	return strings.TrimRight(baseURL, "/") + parsed.Path
}

// ChopString truncates s to maxLen characters.
func ChopString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// collectSelectorTypes returns the unique selector types present across actions.
func collectSelectorTypes(actions []types.EnhancedAction) []string {
	types := make(map[string]bool)
	for _, a := range actions {
		if a.Selectors == nil {
			continue
		}
		for key := range a.Selectors {
			types[key] = true
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	return result
}

// maxFixtures bounds how many routes a script installs.
const maxFixtures = 50

// wantsCheckpoint reports whether the options request a checkpoint after action.
func wantsCheckpoint(opts Params, action types.EnhancedAction) bool {
	return (opts.IncludeScreenshots || opts.VisualAssertions) && action.Type == "navigate" && action.ToURL != ""
}

// writePlaywrightCheckpoint emits the screenshot/assertion lines for checkpoint n.
func writePlaywrightCheckpoint(b *strings.Builder, opts Params, n int) {
	if opts.IncludeScreenshots {
		fmt.Fprintf(b, "  await page.screenshot({ path: 'repro-step-%d.png', fullPage: true });\n", n)
	}
	if opts.VisualAssertions {
		fmt.Fprintf(b, "  await expect(page).toHaveScreenshot('repro-step-%d.png');\n", n)
	}
}

// kaboomCheckpointSteps returns the human-readable checkpoint steps for checkpoint n.
func kaboomCheckpointSteps(opts Params, n int) []string {
	var steps []string
	if opts.IncludeScreenshots {
		steps = append(steps, fmt.Sprintf("Take a screenshot (repro-step-%d.png)", n))
	}
	if opts.VisualAssertions {
		steps = append(steps, fmt.Sprintf("Compare the page against baseline repro-step-%d.png", n))
	}
	return steps
}

// selectFixtures deduplicates by method+URL (latest capture wins, first-seen order
// kept), skips truncated responses, and caps the result. It reports how many
// truncated responses were skipped.
func selectFixtures(bodies []types.NetworkBody, baseURL string) ([]types.NetworkBody, int) {
	index := map[string]int{}
	var selected []types.NetworkBody
	skipped := 0
	for _, body := range bodies {
		if body.ResponseTruncated {
			skipped++
			continue
		}
		if baseURL != "" {
			body.URL = rewriteURL(body.URL, baseURL)
		}
		key := strings.ToUpper(body.Method) + " " + body.URL
		if i, seen := index[key]; seen {
			selected[i] = body
			continue
		}
		if len(selected) >= maxFixtures {
			continue
		}
		index[key] = len(selected)
		selected = append(selected, body)
	}
	return selected, skipped
}

// writePlaywrightFixtures installs a page.route per captured response.
func writePlaywrightFixtures(b *strings.Builder, opts Params) {
	if !opts.GenerateFixtures {
		return
	}
	fixtures, skipped := selectFixtures(opts.Fixtures, opts.BaseURL)
	fmt.Fprintf(b, "  // Network fixtures: %d captured response(s)", len(fixtures))
	if skipped > 0 {
		fmt.Fprintf(b, "; skipped %d truncated", skipped)
	}
	b.WriteString("\n")
	for _, f := range fixtures {
		contentType := f.ContentType
		if contentType == "" {
			contentType = "text/plain"
		}
		fmt.Fprintf(b, "  await page.route('%s', (route) => route.request().method() === '%s' ? route.fulfill({ status: %d, contentType: '%s', body: '%s' }) : route.fallback());\n",
			EscapeJS(f.URL), EscapeJS(strings.ToUpper(f.Method)), f.Status, EscapeJS(contentType), EscapeJS(f.ResponseBody))
	}
}

// writeKaboomFixtures lists the captured responses a replay should stub.
func writeKaboomFixtures(b *strings.Builder, opts Params) {
	if !opts.GenerateFixtures {
		return
	}
	fixtures, skipped := selectFixtures(opts.Fixtures, opts.BaseURL)
	fmt.Fprintf(b, "# Network fixtures (%d captured response(s)", len(fixtures))
	if skipped > 0 {
		fmt.Fprintf(b, "; skipped %d truncated", skipped)
	}
	b.WriteString("):\n")
	for _, f := range fixtures {
		fmt.Fprintf(b, "#   %s %s -> %d\n", strings.ToUpper(f.Method), f.URL, f.Status)
	}
	b.WriteString("\n")
}
