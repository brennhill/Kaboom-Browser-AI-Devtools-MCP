// snippets.go — Reusable tutorial snippet catalog for configure tutorial/examples.
// Why: Keeps static command snippet payloads separate from runtime context analysis logic.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package tutorial

// TutorialSnippets returns the static snippet catalog for tutorial/examples mode.
func TutorialSnippets() []map[string]any {
	return []map[string]any{
		{
			"tool":      "observe",
			"goal":      "Read recent console errors",
			"snippet":   `observe({what:"errors", limit:20})`,
			"arguments": map[string]any{"what": "errors", "limit": 20},
		},
		{
			"tool":      "observe",
			"goal":      "Get latest command result by correlation id",
			"snippet":   `observe({what:"command_result", correlation_id:"corr_123"})`,
			"arguments": map[string]any{"what": "command_result", "correlation_id": "corr_123"},
		},
		{
			"tool":      "interact",
			"goal":      "Navigate to a URL",
			"snippet":   `interact({what:"navigate", url:"https://example.com"})`,
			"arguments": map[string]any{"what": "navigate", "url": "https://example.com"},
		},
		{
			"tool":      "interact",
			"goal":      "Click a button with scoped selector (deterministic)",
			"snippet":   `interact({what:"click", selector:"role=button[name='Submit']", scope_selector:"form[aria-label='Checkout']"})`,
			"arguments": map[string]any{"what": "click", "selector": "role=button[name='Submit']", "scope_selector": "form[aria-label='Checkout']"},
		},
		{
			"tool":      "analyze",
			"goal":      "Get a high-level page summary",
			"snippet":   `analyze({what:"page_summary"})`,
			"arguments": map[string]any{"what": "page_summary"},
		},
		{
			"tool":      "generate",
			"goal":      "Generate a reproduction script from recent actions",
			"snippet":   `generate({what:"reproduction", last_n:20})`,
			"arguments": map[string]any{"what": "reproduction", "last_n": 20},
		},
		{
			"tool":      "configure",
			"goal":      "Check daemon + extension health",
			"snippet":   `configure({what:"health"})`,
			"arguments": map[string]any{"what": "health"},
		},
		{
			"tool":      "configure",
			"goal":      "Inspect tool/mode metadata at runtime",
			"snippet":   `configure({what:"describe_capabilities"})`,
			"arguments": map[string]any{"what": "describe_capabilities"},
		},
		{
			"tool":      "configure",
			"goal":      "Get per-mode params for a specific tool mode",
			"snippet":   `configure({what:"describe_capabilities", tool:"observe", mode:"errors"})`,
			"arguments": map[string]any{"what": "describe_capabilities", "tool": "observe", "mode": "errors"},
		},
	}
}
