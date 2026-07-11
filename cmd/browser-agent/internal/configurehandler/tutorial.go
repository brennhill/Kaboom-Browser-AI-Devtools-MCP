// tutorial.go — Generates context-aware tutorial content for configure(what="tutorial").
// Why: Provides onboarding help and CSP navigation fallback guidance without requiring external documentation.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package configurehandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleTutorial handles configure(what="tutorial") and configure(what="examples").
// failureRecoveryPlaybooks is injected from the caller (lives in playbooks sub-package).
func HandleTutorial(d Deps, req mcp.JSONRPCRequest, args json.RawMessage, failureRecoveryPlaybooks map[string]any) mcp.JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	lenientUnmarshal(args, &params)

	mode := "tutorial"
	if params.What == "examples" {
		mode = "examples"
	}

	context := TutorialContext(d)
	return succeed(req, "Tutorial", map[string]any{
		"status":                     "ok",
		"mode":                       mode,
		"message":                    "Quickstart snippets and context-aware guidance",
		"context":                    context,
		"issues":                     TutorialIssues(context),
		"next_steps":                 TutorialNextSteps(context),
		"snippets":                   TutorialSnippets(),
		"safe_automation_loop":       TutorialSafeAutomationLoop(),
		"csp_fallback_playbook":      TutorialCSPFallbackPlaybook(),
		"failure_recovery_playbooks": failureRecoveryPlaybooks,
		"best_practices": []string{
			"Start with observe to gather evidence before automating actions",
			"Use configure tutorial/examples and describe_capabilities when argument shape is unclear",
			"When debugging, capture correlation_id from interact/analyze and inspect with observe command_result",
			"Use scope + list_interactive + post-action verification to avoid wrong-target clicks",
		},
	})
}

// TutorialContext builds the runtime context map for tutorial responses.
func TutorialContext(d Deps) map[string]any {
	ctx := map[string]any{
		"pilot_enabled":       true,
		"pilot_state":         "assumed_enabled",
		"pilot_authoritative": false,
		"extension_connected": false,
		"tracking_enabled":    false,
		"tracked_tab_id":      0,
		"tracked_tab_url":     "",
	}
	if d == nil {
		return ctx
	}

	trackingEnabled, tabID, tabURL := d.GetTrackingStatus()
	if status, ok := d.GetPilotStatus().(map[string]any); ok {
		if v, ok := status["enabled"].(bool); ok {
			ctx["pilot_enabled"] = v
		}
		if v, ok := status["state"].(string); ok && v != "" {
			ctx["pilot_state"] = v
		}
		if v, ok := status["authoritative"].(bool); ok {
			ctx["pilot_authoritative"] = v
		}
	}
	ctx["extension_connected"] = d.IsExtensionConnected()
	ctx["tracking_enabled"] = trackingEnabled
	ctx["tracked_tab_id"] = tabID
	ctx["tracked_tab_url"] = tabURL
	return ctx
}

// TutorialIssues returns context-aware issue diagnostics for tutorial responses.
func TutorialIssues(context map[string]any) []map[string]any {
	pilotEnabled, _ := context["pilot_enabled"].(bool)
	pilotState, _ := context["pilot_state"].(string)
	extensionConnected, _ := context["extension_connected"].(bool)
	trackingEnabled, _ := context["tracking_enabled"].(bool)
	tabID, _ := context["tracked_tab_id"].(int)
	tabURL, _ := context["tracked_tab_url"].(string)

	issues := make([]map[string]any, 0, 3)
	if pilotState == "explicitly_disabled" || (!pilotEnabled && pilotState == "") {
		issues = append(issues, map[string]any{
			"code":     "pilot_disabled",
			"severity": "warning",
			"message":  "AI Web Pilot is explicitly disabled; interact actions that require extension control will be skipped.",
			"fix":      "Enable AI Web Pilot in the extension popup, then run configure tutorial again.",
			"example":  `configure({what:"doctor"})`,
		})
		return issues
	}

	if !extensionConnected {
		issues = append(issues, map[string]any{
			"code":     "extension_disconnected",
			"severity": "warning",
			"message":  "Extension is not connected; active browser automation and async command polling may fail.",
			"fix":      "Open the extension and verify status is Connected, then retry.",
			"example":  `configure({what:"doctor"})`,
		})
		return issues
	}

	if !trackingEnabled || tabID <= 0 || tabURL == "" {
		issues = append(issues, map[string]any{
			"code":     "no_tracked_tab",
			"severity": "warning",
			"message":  "No tracked tab is active; observe/interact responses may be empty or stale.",
			"fix":      "Track a tab in the extension and run a simple interact navigate call.",
			"example":  `interact({what:"navigate", url:"https://example.com"})`,
		})
	}

	return issues
}

// TutorialNextSteps returns context-aware next step suggestions.
func TutorialNextSteps(context map[string]any) []string {
	issues := TutorialIssues(context)
	if len(issues) > 0 {
		return []string{
			"Run configure doctor to verify environment status",
			"Resolve the warning shown in issues",
			"Retry with a minimal snippet from tutorial snippets",
		}
	}
	return []string{
		"Run observe errors to inspect current page issues",
		"Run interact navigate to move to a target page",
		"Run analyze page_summary for a compact state summary",
	}
}
