// modespecs_generate.go — generate tool per-mode parameter specs.
// Docs: docs/features/generation/describe_capabilities.md
package capabilities

var generateModeSpecs = map[string]modeParamSpec{
	"reproduction": {
		Hint:     "Generate Playwright reproduction script from captured actions/errors",
		Returns:  "A runnable reproduction SCRIPT as text, plus its metadata. The script is returned, not written to disk.",
		Optional: []string{"error_message", "last_n", "base_url", "include_screenshots", "generate_fixtures", "visual_assertions", "output_format", "save_to"},
	},
	"test": {
		Hint:     "Generate Playwright test from recorded browser actions (requires prior action capture)",
		Returns:  "A generated test file as TEXT for you to save, not a file on disk.",
		Optional: []string{"test_name", "last_n", "base_url", "assert_network", "assert_no_errors", "assert_response_shape", "save_to"},
	},
	"pr_summary": {
		Hint:     "Generate PR summary from captured session activity",
		Returns:  "A prose PR summary with change statistics.",
		Optional: []string{"save_to"},
	},
	"har": {
		Hint:     "Export captured network traffic as HAR file",
		Returns:  "A full HAR archive of captured traffic, inline as JSON.",
		Optional: []string{"url", "method", "status_min", "status_max", "save_to"},
	},
	"csp": {
		Hint:     "Generate Content-Security-Policy header from observed resources",
		Returns:  "A suggested Content-Security-Policy header value.",
		Optional: []string{"mode", "include_report_uri", "exclude_origins", "save_to"},
	},
	"sri": {
		Hint:     "Generate Subresource Integrity hashes for scripts/styles",
		Returns:  "Subresource-integrity hashes for the page's external scripts.",
		Optional: []string{"resource_types", "origins", "save_to"},
	},
	"sarif": {
		Hint:     "Export errors and violations as SARIF for IDE/CI integration",
		Returns:  "An accessibility report in SARIF format, inline as JSON.",
		Optional: []string{"scope", "include_passes", "save_to"},
	},
	"visual_test": {
		Hint:     "Generate visual regression test from annotations",
		Returns:  "A generated visual-regression test as text.",
		Optional: []string{"test_name", "annot_session", "save_to"},
	},
	"annotation_report": {
		Hint:     "Generate markdown report from annotation session (markdown prose output)",
		Returns:  "A prose report built from a draw session's annotations.",
		Optional: []string{"annot_session", "save_to"},
	},
	"annotation_issues": {
		Hint:     "Generate structured issue list from annotations (structured JSON output)",
		Returns:  "Annotations converted into filable issue text.",
		Optional: []string{"annot_session", "save_to"},
	},
	"test_from_context": {
		Hint:     "Generate test from error/interaction/regression context. Requires context param: error|interaction|regression",
		Returns:  "A test generated from recent captured activity, as text.",
		Required: []string{"context"},
		Optional: []string{"error_id", "include_mocks", "output_format", "save_to"},
	},
	"test_heal": {
		Hint:     "Analyze or repair broken test selectors. action: analyze (default) | repair | batch",
		Returns:  "Proposed selector repairs for a failing test — suggestions, not edits applied to your files.",
		Optional: []string{"action", "test_file", "test_dir", "broken_selectors", "auto_apply", "save_to"},
	},
	"test_classify": {
		Hint:     "Classify test failures by root cause. action: failure (single) | batch (multiple)",
		Returns:  "A classification of why a test failed.",
		Optional: []string{"action", "failure", "failures", "save_to"},
	},
}
