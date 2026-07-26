// modespecs_generate.go — generate tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package capabilities

var generateModeSpecs = map[string]modeParamSpec{
	"reproduction": {
		Hint:     "Generate Playwright reproduction script from captured actions/errors",
		Optional: []string{"error_message", "last_n", "base_url", "include_screenshots", "generate_fixtures", "visual_assertions", "output_format", "save_to"},
	},
	"test": {
		Hint:     "Generate Playwright test from recorded browser actions (requires prior action capture)",
		Optional: []string{"test_name", "last_n", "base_url", "assert_network", "assert_no_errors", "assert_response_shape", "save_to"},
	},
	"pr_summary": {
		Hint:     "Generate PR summary from captured session activity",
		Optional: []string{"save_to"},
	},
	"har": {
		Hint:     "Export captured network traffic as HAR file",
		Optional: []string{"url", "method", "status_min", "status_max", "save_to"},
	},
	"csp": {
		Hint:     "Generate Content-Security-Policy header from observed resources",
		Optional: []string{"mode", "include_report_uri", "exclude_origins", "save_to"},
	},
	"sri": {
		Hint:     "Generate Subresource Integrity hashes for scripts/styles",
		Optional: []string{"resource_types", "origins", "save_to"},
	},
	"sarif": {
		Hint:     "Export errors and violations as SARIF for IDE/CI integration",
		Optional: []string{"scope", "include_passes", "save_to"},
	},
	"visual_test": {
		Hint:     "Generate visual regression test from annotations",
		Optional: []string{"test_name", "annot_session", "save_to"},
	},
	"annotation_report": {
		Hint:     "Generate markdown report from annotation session (markdown prose output)",
		Optional: []string{"annot_session", "save_to"},
	},
	"annotation_issues": {
		Hint:     "Generate structured issue list from annotations (structured JSON output)",
		Optional: []string{"annot_session", "save_to"},
	},
	"test_from_context": {
		Hint:     "Generate test from error/interaction/regression context. Requires context param: error|interaction|regression",
		Required: []string{"context"},
		Optional: []string{"error_id", "include_mocks", "output_format", "save_to"},
	},
	"test_heal": {
		Hint:     "Analyze or repair broken test selectors. action: analyze (default) | repair | batch",
		Optional: []string{"action", "test_file", "test_dir", "broken_selectors", "auto_apply", "save_to"},
	},
	"test_classify": {
		Hint:     "Classify test failures by root cause. action: failure (single) | batch (multiple)",
		Optional: []string{"action", "failure", "failures", "save_to"},
	},
}
