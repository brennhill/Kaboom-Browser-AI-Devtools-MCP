// modespecs_analyze.go — analyze tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package capabilities

var analyzeModeSpecs = map[string]modeParamSpec{
	"dom": {
		Hint:     "Query DOM elements matching a CSS selector. Omit selector to dump all elements",
		Optional: []string{"selector", "frame", "tab_id"},
	},
	"performance": {
		Hint: "Page load performance metrics and bottleneck analysis",
	},
	"accessibility": {
		Hint:     "WCAG/axe accessibility audit with violation details. summary=true returns counts + top issues",
		Optional: []string{"selector", "scope", "tags", "force_refresh", "frame", "summary"},
	},
	"error_clusters": {
		Hint: "Group errors by pattern to identify systemic issues",
	},
	"navigation_patterns": {
		Hint: "Analyze navigation history patterns and detect repeated loops or dead ends",
	},
	"security_audit": {
		Hint:     "Check for credential leaks, CSP, cookie, and header risks. summary=true returns counts + top issues",
		Optional: []string{"checks", "severity_min", "summary"},
	},
	"third_party_audit": {
		Hint:     "Audit third-party script origins and data exposure. summary=true returns counts + top origins",
		Optional: []string{"first_party_origins", "include_static", "custom_lists", "summary"},
	},
	"link_health": {
		Hint:     "Check all page links for broken URLs (404s, timeouts)",
		Optional: []string{"domain", "max_workers", "timeout_ms"},
	},
	"link_validation": {
		Hint:     "Validate specific URLs for reachability",
		Required: []string{"urls"},
	},
	"page_summary": {
		Hint:     "AI-generated summary of page content and structure (for metadata only use observe/page)",
		Optional: []string{"world", "tab_id", "timeout_ms"},
	},
	"annotations": {
		Hint:     "List annotations from a draw/annotation session. Set background:false (default) to block until annotations arrive (up to timeout_ms)",
		Optional: []string{"annot_session", "background", "timeout_ms", "url"},
	},
	"annotation_detail": {
		Hint:     "Full DOM/style details for a specific annotation",
		Required: []string{"correlation_id"},
	},
	"api_validation": {
		Hint:     "Validate API responses against contract/schema",
		Optional: []string{"operation", "ignore_endpoints"},
	},
	"draw_history": {
		Hint: "List saved annotation/draw sessions",
	},
	"draw_session": {
		Hint:     "Load all annotations from a saved draw session file",
		Required: []string{"file"},
	},
	"computed_styles": {
		Hint:     "CSS computed styles for an element",
		Optional: []string{"selector", "frame"},
	},
	"forms": {
		Hint:     "Form structure: field names, types, and attributes",
		Optional: []string{"selector", "frame"},
	},
	"form_state": {
		Hint:     "Extract current form values and field metadata as structured JSON",
		Optional: []string{"selector", "frame"},
	},
	"form_validation": {
		Hint:     "Check form validation rules and constraint violations. summary=true returns counts only",
		Optional: []string{"summary"},
	},
	"data_table": {
		Hint:     "Extract HTML table data into structured rows/columns",
		Optional: []string{"selector", "max_rows", "max_cols"},
	},
	"visual_baseline": {
		Hint:     "Capture a baseline screenshot for visual regression",
		Required: []string{"name"},
	},
	"visual_diff": {
		Hint:     "Compare current page against a visual baseline",
		Required: []string{"baseline"},
		Optional: []string{"name", "threshold"},
	},
	"visual_baselines": {
		Hint: "List all stored visual regression baselines",
	},
	"navigation": {
		Hint:     "Discover navigable links grouped by page region (nav, header, footer, aside)",
		Optional: []string{"tab_id"},
	},
	"page_structure": {
		Hint:     "Detect frameworks, routing, scroll containers, modals, shadow DOM, and meta tags (structural metadata; for content use page_summary)",
		Optional: []string{"tab_id"},
	},
	"audit": {
		Hint:     "Lighthouse-style combined audit: performance, accessibility, security, best practices",
		Optional: []string{"categories", "summary"},
	},
	"feature_gates": {
		Hint:     "Detect feature flags, A/B tests, and experiment gates in page JavaScript",
		Optional: []string{"tab_id"},
	},
	"page_issues": {
		Hint:     "One-call sweep: aggregates console errors, network failures, a11y violations, and security findings into a unified prioritized report. summary=true returns counts + top issues",
		Optional: []string{"categories", "limit", "summary"},
	},
}
