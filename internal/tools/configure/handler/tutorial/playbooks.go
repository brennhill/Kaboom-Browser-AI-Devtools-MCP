// playbooks.go — Tutorial playbooks for safe automation loops and CSP fallback flows.
// Why: Separates static playbook guidance from configure tutorial runtime logic.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package tutorial

const (
	// CSPRetryNavigationGuidance is the user-facing guidance for CSP-blocked pages.
	CSPRetryNavigationGuidance = "This page blocks script execution (CSP/restricted context). Use interact navigate/refresh/back/forward/new_tab/switch_tab/close_tab to move to another page."
	cspFallbackStatusPattern   = "Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS|ERROR"
)

// TutorialSafeAutomationLoop returns the deterministic safe automation loop playbook.
func TutorialSafeAutomationLoop() map[string]any {
	return map[string]any{
		"title": "Deterministic safe automation loop",
		"steps": []map[string]any{
			{"step": 1, "name": "scope_selection", "instruction": `Pick a scope first (scope_selector or frame) before targeting controls.`},
			{"step": 2, "name": "list_interactive_in_scope", "instruction": `Run interact({what:"list_interactive", ...scope...}) and capture candidate indices/labels.`},
			{"step": 3, "name": "candidate_verification", "instruction": `Verify candidate role/name/label matches intent before acting.`},
			{"step": 4, "name": "action_execution", "instruction": `Execute action using element_id/index when available; avoid broad global selectors.`},
			{"step": 5, "name": "post_action_verification", "instruction": `Verify state changed (DOM/text/url) and optionally capture screenshot evidence.`},
		},
		"bad_vs_good": []map[string]any{
			{
				"action": "submit_post",
				"bad":    `interact({what:"click", selector:"text=Post"})`,
				"good":   `interact({what:"list_interactive", scope_selector:"[role='dialog'][aria-label*='Create post']"}) -> verify candidate -> interact({what:"click", index:2}) -> observe({what:"screenshot", selector:"[data-test='feed-post']"})`,
				"reason": "Global text selectors are ambiguous on complex pages with multiple dialogs/buttons.",
			},
		},
		"scenarios": []map[string]any{
			{
				"id":          "multi_dialog",
				"name":        "Multi-dialog composer flow",
				"description": "When two dialogs are present, always scope to the active composer container before selecting a button.",
				"snippet":     `interact({what:"list_interactive", scope_selector:"[role='dialog'][aria-modal='true']"})`,
			},
			{
				"id":          "iframe",
				"name":        "Iframe-scoped interaction flow",
				"description": "When controls are inside an iframe, set frame first, then list/verify/click in that frame.",
				"snippet":     `interact({what:"list_interactive", frame:"iframe.editor", scope_selector:"form"}) -> interact({what:"click", frame:"iframe.editor", index:1})`,
			},
			{
				"id":          "csp_restricted_page",
				"name":        "CSP-restricted execute_js fallback flow",
				"description": "If execute_js reports CSP failure, switch to DOM primitives + scope + screenshot verification instead of retrying arbitrary JS.",
				"snippet":     `interact({what:"execute_js", script:"document.title", world:"main", background:true}) -> observe({what:"command_result", correlation_id:"<corr>"}) -> interact({what:"list_interactive", scope_selector:"main"}) -> interact({what:"click", index:1}) -> observe({what:"screenshot"})`,
			},
		},
	}
}

// TutorialCSPFallbackPlaybook returns the CSP-safe automation playbook.
func TutorialCSPFallbackPlaybook() map[string]any {
	return map[string]any{
		"title":                   "CSP-safe automation playbook (execute_js fallback)",
		"detect_signals":          []string{"error=csp_blocked_all_worlds", "failure_cause=csp", "csp_blocked=true"},
		"fallback_status_pattern": cspFallbackStatusPattern,
		"exact_retry_guidance":    CSPRetryNavigationGuidance,
		"what_is_possible": []string{
			"Pre-compiled DOM primitives (click/type/select/check/focus/list_interactive/highlight)",
			"DOM inspection and screenshot checkpoints",
			"Navigation escape actions (navigate/refresh/back/forward/new_tab/switch_tab/close_tab)",
		},
		"what_is_not_possible": []string{
			"Arbitrary page-context JS eval when CSP/Trusted Types blocks dynamic script execution",
			"Assuming MAIN world and ISOLATED world have identical capabilities on every page",
		},
		"fallback_sequence": []map[string]any{
			{"step": 1, "instruction": `Detect CSP in observe(command_result): error=csp_blocked_all_worlds or failure_cause=csp.`},
			{"step": 2, "instruction": `Stop retrying execute_js on the same page context. Switch to list_interactive + scoped DOM primitives.`},
			{"step": 3, "instruction": `Run the action with scope/frame constraints and verify the intended target before clicking.`},
			{"step": 4, "instruction": `Capture screenshot evidence after action to prove post-condition.`},
		},
		"command_examples": []map[string]any{
			{
				"goal":     "Detect CSP failure and capture retry guidance",
				"snippet":  `observe({what:"command_result", correlation_id:"<corr>"})`,
				"expected": `error=csp_blocked_all_worlds | failure_cause=csp | retry=` + CSPRetryNavigationGuidance,
			},
			{
				"goal":     "Run CSP-safe fallback flow",
				"snippet":  `interact({what:"list_interactive", scope_selector:"main"}) -> interact({what:"click", index:1}) -> observe({what:"screenshot"})`,
				"expected": "Deterministic target selection and visual checkpoint without arbitrary JS eval",
			},
		},
	}
}
