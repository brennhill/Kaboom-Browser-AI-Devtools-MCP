// Purpose: Validate launch-mode classification and strict-mode policy behavior.
// Why: Prevents regressions in startup warnings and strict persistent-launch enforcement.

package launchmode

import (
	"strings"
	"testing"
)

// clearSupervisorEnv neutralizes every supervisor marker so Classify
// does not treat the test host (e.g. CI under systemd) as supervised.
func clearSupervisorEnv(t *testing.T) {
	t.Helper()
	t.Setenv("KABOOM_SUPERVISED", "")
	for _, k := range supervisorEnvVars {
		t.Setenv(k, "")
	}
}

func TestClassifyLaunchMode_DaemonFlagAlwaysPersistent(t *testing.T) {
	info := Classify(true, true, "zsh")
	if info.Mode != Persistent {
		t.Fatalf("mode = %q, want %q", info.Mode, Persistent)
	}
	if info.Reason != "daemon_flag_enabled" {
		t.Fatalf("reason = %q, want daemon_flag_enabled", info.Reason)
	}
}

func TestClassifyLaunchMode_InteractiveShellIsLikelyTransient(t *testing.T) {
	clearSupervisorEnv(t)

	info := Classify(false, true, "/bin/zsh")
	if info.Mode != LikelyTransient {
		t.Fatalf("mode = %q, want %q", info.Mode, LikelyTransient)
	}
	if info.Reason != "interactive_shell_parent" {
		t.Fatalf("reason = %q, want interactive_shell_parent", info.Reason)
	}
}

func TestClassifyLaunchMode_NonInteractiveDefaultsPersistent(t *testing.T) {
	clearSupervisorEnv(t)

	info := Classify(false, false, "")
	if info.Mode != Persistent {
		t.Fatalf("mode = %q, want %q", info.Mode, Persistent)
	}
	if info.Reason != "non_interactive_stdio" {
		t.Fatalf("reason = %q, want non_interactive_stdio", info.Reason)
	}
}

func TestEnforcePersistentMode_StrictTransientFails(t *testing.T) {
	info := Info{
		Mode:           LikelyTransient,
		Reason:         "interactive_shell_parent",
		StrictRequired: true,
	}
	err := EnforcePersistent(info, 7890)
	if err == nil {
		t.Fatal("expected strict-mode error for likely_transient launch")
	}
	if !strings.Contains(err.Error(), "KABOOM_REQUIRE_PERSISTENT") {
		t.Fatalf("error = %q, expected strict-mode guidance", err.Error())
	}
}

func TestBuildLaunchModeWarning_ContainsRemediation(t *testing.T) {
	warn := Warning(Info{
		Mode:   LikelyTransient,
		Reason: "interactive_shell_parent",
	}, 7890)
	if warn == "" {
		t.Fatal("expected warning text")
	}
	if !strings.Contains(warn, "kaboom-agentic-browser --daemon --port 7890") {
		t.Fatalf("warning = %q, expected remediation command", warn)
	}
}
