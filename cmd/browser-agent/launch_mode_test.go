// Purpose: Validate launch_mode.go classification and strict-mode policy behavior.
// Why: Prevents regressions in startup warnings and strict persistent-launch enforcement.

package main

import (
	"strings"
	"testing"
)

// clearSupervisorEnv neutralizes every supervisor marker so classifyLaunchMode
// does not treat the test host (e.g. CI under systemd) as supervised.
func clearSupervisorEnv(t *testing.T) {
	t.Helper()
	t.Setenv("KABOOM_SUPERVISED", "")
	for _, k := range supervisorEnvVars {
		t.Setenv(k, "")
	}
}

func TestClassifyLaunchMode_DaemonFlagAlwaysPersistent(t *testing.T) {
	origLookup := lookupParentProcessName
	t.Cleanup(func() { lookupParentProcessName = origLookup })
	lookupParentProcessName = func() string { return "zsh" }

	info := classifyLaunchMode(&serverConfig{daemonMode: true}, true)
	if info.Mode != launchModePersistent {
		t.Fatalf("mode = %q, want %q", info.Mode, launchModePersistent)
	}
	if info.Reason != "daemon_flag_enabled" {
		t.Fatalf("reason = %q, want daemon_flag_enabled", info.Reason)
	}
}

func TestClassifyLaunchMode_InteractiveShellIsLikelyTransient(t *testing.T) {
	clearSupervisorEnv(t)

	origLookup := lookupParentProcessName
	t.Cleanup(func() { lookupParentProcessName = origLookup })
	lookupParentProcessName = func() string { return "/bin/zsh" }

	info := classifyLaunchMode(&serverConfig{}, true)
	if info.Mode != launchModeLikelyTransient {
		t.Fatalf("mode = %q, want %q", info.Mode, launchModeLikelyTransient)
	}
	if info.Reason != "interactive_shell_parent" {
		t.Fatalf("reason = %q, want interactive_shell_parent", info.Reason)
	}
}

func TestClassifyLaunchMode_NonInteractiveDefaultsPersistent(t *testing.T) {
	clearSupervisorEnv(t)

	origLookup := lookupParentProcessName
	t.Cleanup(func() { lookupParentProcessName = origLookup })
	lookupParentProcessName = func() string { return "" }

	info := classifyLaunchMode(&serverConfig{}, false)
	if info.Mode != launchModePersistent {
		t.Fatalf("mode = %q, want %q", info.Mode, launchModePersistent)
	}
	if info.Reason != "non_interactive_stdio" {
		t.Fatalf("reason = %q, want non_interactive_stdio", info.Reason)
	}
}

func TestEnforcePersistentMode_StrictTransientFails(t *testing.T) {
	info := launchModeInfo{
		Mode:           launchModeLikelyTransient,
		Reason:         "interactive_shell_parent",
		StrictRequired: true,
	}
	err := enforcePersistentMode(info)
	if err == nil {
		t.Fatal("expected strict-mode error for likely_transient launch")
	}
	if !strings.Contains(err.Error(), "KABOOM_REQUIRE_PERSISTENT") {
		t.Fatalf("error = %q, expected strict-mode guidance", err.Error())
	}
}

func TestBuildLaunchModeWarning_ContainsRemediation(t *testing.T) {
	warn := buildLaunchModeWarning(launchModeInfo{
		Mode:   launchModeLikelyTransient,
		Reason: "interactive_shell_parent",
	}, 7890)
	if warn == "" {
		t.Fatal("expected warning text")
	}
	if !strings.Contains(warn, "kaboom-agentic-browser --daemon --port 7890") {
		t.Fatalf("warning = %q, expected remediation command", warn)
	}
}
