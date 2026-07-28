// Purpose: Tests for configure tutorial content delivery.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_configure_tutorial_test.go — Tests for configure tutorial mode.
package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func seedSyncSettings(t *testing.T, env *configureTestEnv, settingsJSON string) {
	t.Helper()
	reqBody := `{"ext_session_id":"tutorial-test","settings":` + settingsJSON + `}`
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(reqBody))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(env.capture).HandleSync(httptest.NewRecorder(), httpReq)
}

func TestToolsConfigureTutorial_ResponseShape(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if data["status"] != "ok" {
		t.Fatalf("status = %v, want ok", data["status"])
	}
	if data["mode"] != "tutorial" {
		t.Fatalf("mode = %v, want tutorial", data["mode"])
	}
	if _, ok := data["snippets"].([]any); !ok {
		t.Fatalf("snippets type = %T, want []any", data["snippets"])
	}
	if _, ok := data["context"].(map[string]any); !ok {
		t.Fatalf("context type = %T, want map[string]any", data["context"])
	}
	if _, ok := data["issues"].([]any); !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
}

func TestToolsConfigureExamples_IsRejected(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"examples"}`)
	if !ok {
		t.Fatal("examples should return result")
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Fatalf("examples alias should be rejected, got: %s", result.Content[0].Text)
	}
}

func TestToolsConfigureTutorial_ContextAware_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}
	data := parseResponseJSON(t, result)

	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	if len(issues) == 0 {
		t.Fatal("expected at least one context issue when pilot is disabled")
	}
	first, _ := issues[0].(map[string]any)
	if first["code"] != "pilot_disabled" {
		t.Fatalf("first issue code = %v, want pilot_disabled", first["code"])
	}
}

func TestToolsConfigureTutorial_ContextAware_AssumedPilotDoesNotReportPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)
	env.capture.Extension().SetPilotUnknownForTest()

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}
	data := parseResponseJSON(t, result)

	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	for _, issueRaw := range issues {
		issue, _ := issueRaw.(map[string]any)
		if issue["code"] == "pilot_disabled" {
			t.Fatalf("unexpected pilot_disabled issue while pilot state is only assumed at startup: %+v", issue)
		}
	}
}

func TestToolsConfigureTutorial_ContextAware_NoTrackedTab(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	seedSyncSettings(t, env, `{"pilot_enabled":true,"tracking_enabled":false,"tracked_tab_id":0,"tracked_tab_url":"","tracked_tab_title":""}`)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}
	data := parseResponseJSON(t, result)

	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	found := false
	for _, issueRaw := range issues {
		issue, _ := issueRaw.(map[string]any)
		if issue["code"] == "no_tracked_tab" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected no_tracked_tab issue, got: %+v", issues)
	}
}

func TestToolsConfigureTutorial_IncludesSafeAutomationLoop(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	seedSyncSettings(t, env, `{"pilot_enabled":true,"tracking_enabled":true,"tracked_tab_id":11,"tracked_tab_url":"https://example.com","tracked_tab_title":"Example"}`)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	loop, ok := data["safe_automation_loop"].(map[string]any)
	if !ok {
		t.Fatalf("safe_automation_loop type = %T, want map[string]any", data["safe_automation_loop"])
	}
	steps, ok := loop["steps"].([]any)
	if !ok || len(steps) < 5 {
		t.Fatalf("safe_automation_loop.steps = %#v, want >=5 steps", loop["steps"])
	}
	badVsGood, ok := loop["bad_vs_good"].([]any)
	if !ok || len(badVsGood) == 0 {
		t.Fatalf("safe_automation_loop.bad_vs_good = %#v, want non-empty", loop["bad_vs_good"])
	}
	scenarios, ok := loop["scenarios"].([]any)
	if !ok || len(scenarios) < 2 {
		t.Fatalf("safe_automation_loop.scenarios = %#v, want at least 2", loop["scenarios"])
	}
	ids := make(map[string]bool, len(scenarios))
	for _, raw := range scenarios {
		entry, _ := raw.(map[string]any)
		id, _ := entry["id"].(string)
		ids[id] = true
	}
	if !ids["multi_dialog"] {
		t.Fatalf("safe_automation_loop missing multi_dialog scenario: %#v", scenarios)
	}
	if !ids["iframe"] {
		t.Fatalf("safe_automation_loop missing iframe scenario: %#v", scenarios)
	}
	if !ids["csp_restricted_page"] {
		t.Fatalf("safe_automation_loop missing csp_restricted_page scenario: %#v", scenarios)
	}
}

func TestToolsConfigureTutorial_IncludesCSPFallbackPlaybook(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	seedSyncSettings(t, env, `{"pilot_enabled":true,"tracking_enabled":true,"tracked_tab_id":17,"tracked_tab_url":"https://example.com","tracked_tab_title":"Example"}`)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	playbook, ok := data["csp_fallback_playbook"].(map[string]any)
	if !ok {
		t.Fatalf("csp_fallback_playbook type = %T, want map[string]any", data["csp_fallback_playbook"])
	}

	detectSignals, ok := playbook["detect_signals"].([]any)
	if !ok || len(detectSignals) == 0 {
		t.Fatalf("csp_fallback_playbook.detect_signals = %#v, want non-empty []any", playbook["detect_signals"])
	}
	hasCSPBlockedAllWorlds := false
	hasFailureCause := false
	for _, raw := range detectSignals {
		signal, _ := raw.(string)
		if signal == "error=csp_blocked_all_worlds" {
			hasCSPBlockedAllWorlds = true
		}
		if signal == "failure_cause=csp" {
			hasFailureCause = true
		}
	}
	if !hasCSPBlockedAllWorlds {
		t.Fatalf("detect_signals missing error=csp_blocked_all_worlds: %#v", detectSignals)
	}
	if !hasFailureCause {
		t.Fatalf("detect_signals missing failure_cause=csp: %#v", detectSignals)
	}

	retryGuidance, _ := playbook["exact_retry_guidance"].(string)
	expectedRetry := "This page blocks script execution (CSP/restricted context). Use interact navigate/refresh/back/forward/new_tab/switch_tab/close_tab to move to another page."
	if retryGuidance != expectedRetry {
		t.Fatalf("exact_retry_guidance = %q, want %q", retryGuidance, expectedRetry)
	}

	errorPattern, _ := playbook["fallback_status_pattern"].(string)
	expectedPattern := "Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS|ERROR"
	if errorPattern != expectedPattern {
		t.Fatalf("fallback_status_pattern = %q, want %q", errorPattern, expectedPattern)
	}
}

func TestToolsConfigureTutorial_SnippetsAvoidAmbiguousGlobalSubmit(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	snippets, ok := data["snippets"].([]any)
	if !ok {
		t.Fatalf("snippets type = %T, want []any", data["snippets"])
	}
	for _, raw := range snippets {
		entry, _ := raw.(map[string]any)
		snippet, _ := entry["snippet"].(string)
		if strings.Contains(snippet, `selector:"text=Submit"`) {
			t.Fatalf("found ambiguous global submit snippet: %s", snippet)
		}
	}
}

func TestToolsConfigureTutorial_IncludesFailureRecoveryPlaybooks(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)
	seedSyncSettings(t, env, `{"pilot_enabled":true,"tracking_enabled":true,"tracked_tab_id":19,"tracked_tab_url":"https://example.com","tracked_tab_title":"Example"}`)

	result, ok := env.callConfigure(t, `{"what":"tutorial"}`)
	if !ok {
		t.Fatal("tutorial should return result")
	}
	if result.IsError {
		t.Fatalf("tutorial should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	playbooks, ok := data["failure_recovery_playbooks"].(map[string]any)
	if !ok {
		t.Fatalf("failure_recovery_playbooks type = %T, want map[string]any", data["failure_recovery_playbooks"])
	}

	required := []string{"element_not_found", "ambiguous_target", "stale_element_id", "scope_not_found", "blocked_by_overlay"}
	for _, code := range required {
		raw, ok := playbooks[code]
		if !ok {
			t.Fatalf("missing playbook for %s: %#v", code, playbooks)
		}
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("playbook[%s] type = %T, want map[string]any", code, raw)
		}
		if detect, _ := entry["detection_signal"].(string); detect == "" {
			t.Fatalf("playbook[%s].detection_signal is empty", code)
		}
		steps, ok := entry["ordered_recovery_steps"].([]any)
		if !ok || len(steps) < 2 {
			t.Fatalf("playbook[%s].ordered_recovery_steps = %#v, want >=2 steps", code, entry["ordered_recovery_steps"])
		}
		if stop, _ := entry["stop_and_report_condition"].(string); stop == "" {
			t.Fatalf("playbook[%s].stop_and_report_condition is empty", code)
		}
	}
}
