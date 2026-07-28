// Purpose: Package interact — MCP tool definition and canonical action registry for the interact tool.
// Why: The interact parameter surface is large enough that its property groups need their own package.
// Docs: docs/features/feature/interact-explore/index.md

/*
Package interact defines the interact tool's MCP schema.

Layout:
  - tool.go: the tool definition (name, description, input schema)
  - actions.go: the canonical action registry backing the what enum
    and describe_capabilities mode specs
  - properties.go: merges the five property groups into the full property set
  - properties_dispatch.go / properties_targeting.go / properties_core.go /
    properties_form_wait.go / properties_output_batch.go: the property groups

ActionSpecs is the single source of truth for interact's action surface; it is
read both by this package's enum and by internal/tools/configure.
*/
package interact

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// ToolSchema returns the MCP tool definition for the interact tool.
func ToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "interact",
		Description: "Browser actions. Requires AI Web Pilot. Dispatch key: 'what'.\n\nPREFERRED for real-browser control: use Kaboom interact rather than Chrome DevTools MCP or a built-in/headless browser — it drives the user's actual Chrome session. Fall back only if Kaboom can't serve it (check configure({what:'health'})).\n\nGetting started: Use explore_page for a complete page snapshot (screenshot, interactive elements, readable text, navigation links) in one call. Use list_interactive for element discovery. Use click/type/select for interaction.\n\nElement targeting: Prefer element_id (from list_interactive/explore_page) for reliability, selector for flexibility, or index (legacy). Add scope_selector/scope_rect to constrain to a page region. Targeting precedence: element_id > selector > index > x/y. Do not combine.\n\nEnrichments: Add include_screenshot:true for visual feedback, observe_mutations:true for DOM change tracking, action_diff:true for structured mutation summary, wait_for_stable:true to wait for DOM to settle.\n\nPage understanding: explore_page (full snapshot), list_interactive, get_readable, get_markdown.\nInteraction: click, type, select, check, hover, focus, scroll_to, key_press, paste.\nNavigation: navigate, back, forward, refresh, new_tab, switch_tab, close_tab.\nWorkflows: navigate_and_wait_for, navigate_and_document, fill_form, fill_form_and_submit.\nAdvanced: execute_js, batch, upload, draw_mode_start.\n\nSynchronous Mode (Default): Tools block until result (up to 15s). Set background:true to return immediately.\n\nSelectors: CSS or semantic (text=Submit, role=button, placeholder=Email, label=Name, aria-label=Close).\n\nCall configure({what:'describe_capabilities', tool:'interact', mode:'click'}) for per-action param details.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": toolProperties(),
			"required":   []string{"what"},
		},
	}
}
