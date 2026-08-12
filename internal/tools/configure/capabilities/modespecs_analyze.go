// modespecs_analyze.go — analyze tool per-mode parameter specs.
// Docs: docs/features/generation/describe_capabilities.md
package capabilities

var analyzeModeSpecs = map[string]modeParamSpec{
	"dom": {
		Hint:     "Query DOM elements matching a CSS selector. Omit selector to dump all elements",
		Returns:  "The elements matching your selector, with their attributes and text.",
		Optional: []string{"selector", "frame", "tab_id"},
	},
	"performance": {
		Hint:     "Page load metrics, ordered critical-path analysis, and optional local backend trace correlation",
		Returns:  "Performance snapshots with a critical-path breakdown.",
		Optional: []string{"trace_source"},
	},
	"performance_trace": {
		Hint:     "Record a full Chrome CPU flamechart trace to a local DevTools/Perfetto-importable JSON artifact",
		Returns:  "A recorded Chrome performance trace for one tab.",
		Required: []string{"action"},
		Optional: []string{"tab_id", "reload", "cache", "background"},
	},
	"react_profile": {
		Hint:     "Opt-in React commit profiling; start, exercise the page, then stop to retrieve bounded component evidence",
		Returns:  "A React render profile for one tab.",
		Required: []string{"action"},
		Optional: []string{"tab_id", "background"},
	},
	"accessibility": {
		Hint:     "WCAG/axe accessibility audit with violation details. summary=true returns counts + top issues",
		Returns:  "A list of accessibility violations with the rule each one breaks.",
		Optional: []string{"selector", "scope", "tags", "force_refresh", "frame", "summary"},
	},
	"error_clusters": {
		Hint:    "Group errors by pattern to identify systemic issues",
		Returns: "Errors grouped by shared pattern, so repeats collapse into one entry.",
	},
	"navigation_patterns": {
		Hint:    "Analyze navigation history patterns and detect repeated loops or dead ends",
		Returns: "The navigation paths users took, aggregated.",
	},
	"security_audit": {
		Hint:     "Check for credential leaks, CSP, cookie, and header risks. summary=true returns counts + top issues",
		Returns:  "A list of security findings with severities.",
		Optional: []string{"checks", "severity_min", "summary"},
	},
	"third_party_audit": {
		Hint:     "Audit third-party script origins and data exposure. summary=true returns counts + top origins",
		Returns:  "The third-party origins the page contacts, with recommendations.",
		Optional: []string{"first_party_origins", "include_static", "custom_lists", "summary"},
	},
	"link_health": {
		Hint:     "Check all page links for broken URLs (404s, timeouts)",
		Returns:  "A list of links with their reachability status.",
		Optional: []string{"domain", "max_workers", "timeout_ms"},
	},
	"link_validation": {
		Hint:     "Validate specific URLs for reachability",
		Returns:  "The reachability result for each URL you supplied.",
		Required: []string{"urls"},
	},
	"page_summary": {
		Hint:     "AI-generated summary of page content and structure (for metadata only use observe/page)",
		Returns:  "A short prose summary of what the page is.",
		Optional: []string{"world", "tab_id", "timeout_ms"},
	},
	"annotations": {
		Hint:     "List annotations from a draw/annotation session. Set background:false (default) to block until annotations arrive (up to timeout_ms)",
		Returns:  "A list of annotations captured in a draw session.",
		Optional: []string{"annot_session", "background", "timeout_ms", "url"},
	},
	"annotation_detail": {
		Hint:     "Full DOM/style details for a specific annotation",
		Returns:  "The full DOM and style detail behind ONE annotation.",
		Required: []string{"correlation_id"},
	},
	"api_validation": {
		Hint:     "Validate API responses against contract/schema",
		Returns:  "Whether observed API responses matched their contract.",
		Optional: []string{"operation", "ignore_endpoints"},
	},
	"draw_history": {
		Hint:    "List saved annotation/draw sessions",
		Returns: "A list of saved draw-session FILES with names and sizes. Paths only — load one with draw_session.",
	},
	"draw_session": {
		Hint:     "Load all annotations from a saved draw session file",
		Returns:  "The annotations inside ONE saved draw-session file.",
		Required: []string{"file"},
	},
	"computed_styles": {
		Hint:     "CSS computed styles for an element",
		Returns:  "The computed CSS properties and box model for the matched elements.",
		Optional: []string{"selector", "frame"},
	},
	"forms": {
		Hint:     "Form structure: field names, types, and attributes",
		Returns:  "The forms on the page with their fields and types.",
		Optional: []string{"selector", "frame"},
	},
	"form_state": {
		Hint:     "Extract current form values and field metadata as structured JSON",
		Returns:  "The current values held in a form.",
		Optional: []string{"selector", "frame"},
	},
	"form_validation": {
		Hint:     "Check form validation rules and constraint violations. summary=true returns counts only",
		Returns:  "A form's validation state and any messages.",
		Optional: []string{"summary"},
	},
	"data_table": {
		Hint:     "Extract HTML table data into structured rows/columns",
		Returns:  "An HTML table read out as structured rows and columns.",
		Optional: []string{"selector", "max_rows", "max_cols"},
	},
	"visual_baseline": {
		Hint:     "Capture a baseline screenshot for visual regression",
		Returns:  "A FILE PATH to the baseline screenshot just saved.",
		Required: []string{"name"},
	},
	"visual_diff": {
		Hint:     "Compare current page against a visual baseline",
		Returns:  "A verdict plus how many pixels changed against the baseline, and where.",
		Required: []string{"baseline"},
		Optional: []string{"name", "threshold"},
	},
	"visual_baselines": {
		Hint:    "List all stored visual regression baselines",
		Returns: "A list of baseline NAMES that exist. Names only, no images.",
	},
	"navigation": {
		Hint:     "Discover navigable links grouped by page region (nav, header, footer, aside)",
		Returns:  "The page's links grouped by region.",
		Optional: []string{"tab_id"},
	},
	"page_structure": {
		Hint:     "Detect frameworks, routing, scroll containers, modals, shadow DOM, and meta tags (structural metadata; for content use page_summary)",
		Returns:  "What the page is built from — frameworks, routing, modals, shadow roots.",
		Optional: []string{"tab_id"},
	},
	"audit": {
		Hint:     "Lighthouse-style combined audit: performance, accessibility, security, best practices",
		Returns:  "Category scores and recommendations across performance, accessibility and security.",
		Optional: []string{"categories", "summary"},
	},
	"feature_gates": {
		Hint:     "Detect feature flags, A/B tests, and experiment gates in page JavaScript",
		Returns:  "The feature flags, plan gates and usage limits detected on the page.",
		Optional: []string{"tab_id"},
	},
	"page_issues": {
		Hint:     "One-call sweep: aggregates console errors, network failures, a11y violations, and security findings into a unified prioritized report. summary=true returns counts + top issues",
		Returns:  "Everything currently wrong with the page, grouped by category and severity.",
		Optional: []string{"categories", "limit", "summary"},
	},
	"verification": {
		Hint:     "Define or evaluate a versioned QA contract; missing required evidence produces UNVERIFIED",
		Returns:  "Whether a declared verification contract holds, assertion by assertion.",
		Required: []string{"operation", "contract"},
		Optional: []string{"results", "evidence", "evidence_catalog", "max_age_seconds"},
	},
}
