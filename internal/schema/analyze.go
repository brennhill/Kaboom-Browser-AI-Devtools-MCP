// Purpose: Returns the MCP tool definition (name, description, input schema) for the analyze tool.
// Docs: docs/features/feature/analyze-tool/index.md

package schema

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// analyzeToolSchema returns the MCP tool definition for the analyze tool.
func analyzeToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "analyze",
		Description: "Trigger active analysis. Creates async queries the extension executes.\n\nSynchronous Mode (Default): Tools block until the extension returns a result (up to 15s). Set background:true to return immediately with a correlation_id, then poll with observe(what='command_result', correlation_id=...).\n\nDraw Mode: Use annotations to get all annotations from the last draw mode session. Use annotation_detail with correlation_id to get full computed styles and DOM detail for a specific annotation.\n\nUse summary:true on supported modes for compact token-efficient responses.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type":        "string",
					"description": "Analysis mode to run against the page",
					"enum":        []string{"dom", "performance", "accessibility", "error_clusters", "navigation_patterns", "security_audit", "third_party_audit", "link_health", "link_validation", "page_summary", "annotations", "annotation_detail", "api_validation", "draw_history", "draw_session", "computed_styles", "forms", "form_state", "form_validation", "data_table", "visual_baseline", "visual_diff", "visual_baselines", "navigation", "page_structure", "audit", "feature_gates", "page_issues"},
				},
				"telemetry_mode": map[string]any{
					"type":        "string",
					"description": "Telemetry metadata mode for this call: off, auto, full",
					"enum":        []string{"off", "auto", "full"},
				},
				"selector": map[string]any{
					"type":        "string",
					"description": "CSS selector (dom, accessibility, computed_styles, forms, form_state, data_table)",
				},
				"frame": map[string]any{
					"description": "Target iframe: CSS selector, 0-based index, or \"all\" (dom, accessibility)",
					"type":        "string",
				},
				"background": map[string]any{
					"type":        "boolean",
					"description": "Return immediately with correlation_id instead of waiting for result (default: false). For annotations, background=false blocks up to timeout_ms before falling back to correlation polling.",
				},
				"operation": map[string]any{
					"type":        "string",
					"description": "Operation selector (api_validation: analyze/report/clear; annotations: flush)",
					"enum":        []string{"analyze", "report", "clear", "flush"},
				},
				"ignore_endpoints": map[string]any{
					"type":        "array",
					"description": "URL substrings to exclude (api_validation)",
					"items":       map[string]any{"type": "string"},
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "CSS selector scope (accessibility)",
				},
				"tags": map[string]any{
					"type":        "array",
					"description": "WCAG tags (accessibility)",
					"items":       map[string]any{"type": "string"},
				},
				"force_refresh": map[string]any{
					"type":        "boolean",
					"description": "Bypass cache (accessibility)",
				},
				"domain": map[string]any{
					"type":        "string",
					"description": "Domain to check (link_health)",
				},
				"timeout_ms": map[string]any{
					"type":        "number",
					"description": "Timeout in milliseconds. Applies to: link_health (default 10000), page_summary (default 5000), annotations (default 15000, max 600000).",
				},
				"world": map[string]any{
					"type":        "string",
					"description": "Execution world for page_summary script",
					"enum":        []string{"auto", "main", "isolated"},
				},
				"tab_id": map[string]any{
					"type":        "number",
					"description": "Target tab ID (dom, page_summary)",
				},
				"max_workers": map[string]any{
					"type":        "number",
					"description": "Max concurrent workers (link_health)",
				},
				"max_rows": map[string]any{
					"type":        "number",
					"description": "Max rows to return per table (data_table)",
				},
				"max_cols": map[string]any{
					"type":        "number",
					"description": "Max columns to return per table (data_table)",
				},
				"checks": map[string]any{
					"type":        "array",
					"description": "Checks to run (security_audit)",
					"items": map[string]any{
						"type": "string",
						"enum": []string{"credentials", "pii", "headers", "cookies", "transport", "auth"},
					},
				},
				"severity_min": map[string]any{
					"type":        "string",
					"description": "Min severity (security_audit)",
					"enum":        []string{"critical", "high", "medium", "low", "info"},
				},
				"first_party_origins": map[string]any{
					"type":        "array",
					"description": "First-party origins (third_party_audit)",
					"items":       map[string]any{"type": "string"},
				},
				"include_static": map[string]any{
					"type":        "boolean",
					"description": "Include static-only origins (third_party_audit)",
				},
				"custom_lists": map[string]any{
					"type":        "object",
					"description": "Custom domain allow/block lists (third_party_audit)",
				},
				"correlation_id": map[string]any{
					"type":        "string",
					"description": "Correlation ID for fetching annotation detail (applies to annotation_detail)",
				},
				"annot_session": map[string]any{
					"type":        "string",
					"description": "Named session for multi-page annotation review (applies to annotations). Accumulates annotations across pages.",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "Annotation URL scope filter (annotations). Supports exact URL, project base URL, or wildcard patterns such as http://localhost:3000/*.",
				},
				"url_pattern": map[string]any{
					"type":        "string",
					"description": "Alias for annotation URL scope filter (annotations). Use either url or url_pattern.",
				},
				"urls": map[string]any{
					"type":        "array",
					"description": "URLs to validate (link_validation)",
					"items":       map[string]any{"type": "string"},
				},
				"file": map[string]any{
					"type":        "string",
					"description": "Session filename from draw_history results (draw_session)",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Baseline name (visual_baseline, visual_diff)",
				},
				"baseline": map[string]any{
					"type":        "string",
					"description": "Baseline name to compare against (visual_diff)",
				},
				"threshold": map[string]any{
					"type":        "number",
					"description": "Pixel diff threshold 0-255 (visual_diff, default 30)",
				},
				"summary": map[string]any{
					"type":        "boolean",
					"description": "Return compact summary instead of full details (accessibility, security_audit, third_party_audit, form_validation, audit, page_issues)",
				},
				"categories": map[string]any{
					"type":        "array",
					"description": "Categories to include (audit: performance, accessibility, security, best_practices; page_issues: console_errors, network_failures, accessibility, security)",
					"items":       map[string]any{"type": "string"},
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Max issues per section (page_issues, default 50)",
				},
			},
			"required": []string{"what"},
		},
	}
}
