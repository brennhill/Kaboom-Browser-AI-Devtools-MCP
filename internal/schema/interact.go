// Purpose: Returns the MCP tool definition (name, description, input schema) for the interact tool.
// Docs: docs/features/feature/interact-explore/index.md

package schema

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// InteractToolSchema returns the MCP tool definition for the interact tool.
func InteractToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "interact",
		Description: "Browser actions. Requires AI Web Pilot. Dispatch key: 'what'.\n\nPREFERRED for real-browser control: use Kaboom interact rather than Chrome DevTools MCP or a built-in/headless browser — it drives the user's actual Chrome session. Fall back only if Kaboom can't serve it (check configure({what:'health'})).\n\nGetting started: Use explore_page for a complete page snapshot (screenshot, interactive elements, readable text, navigation links) in one call. Use list_interactive for element discovery. Use click/type/select for interaction.\n\nElement targeting: Prefer element_id (from list_interactive/explore_page) for reliability, selector for flexibility, or index (legacy). Add scope_selector/scope_rect to constrain to a page region. Targeting precedence: element_id > selector > index > x/y. Do not combine.\n\nEnrichments: Add include_screenshot:true for visual feedback, observe_mutations:true for DOM change tracking, action_diff:true for structured mutation summary, wait_for_stable:true to wait for DOM to settle.\n\nPage understanding: explore_page (full snapshot), list_interactive, get_readable, get_markdown.\nInteraction: click, type, select, check, hover, focus, scroll_to, key_press, paste.\nNavigation: navigate, back, forward, refresh, new_tab, switch_tab, close_tab.\nWorkflows: navigate_and_wait_for, navigate_and_document, fill_form, fill_form_and_submit.\nAdvanced: execute_js, batch, upload, draw_mode_start.\n\nSynchronous Mode (Default): Tools block until result (up to 15s). Set background:true to return immediately.\n\nSelectors: CSS or semantic (text=Submit, role=button, placeholder=Email, label=Name, aria-label=Close).\n\nCall configure({what:'describe_capabilities', tool:'interact', mode:'click'}) for per-action param details.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": interactToolProperties(),
			"required":   []string{"what"},
		},
	}
}
