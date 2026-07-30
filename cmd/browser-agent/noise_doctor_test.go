// noise_doctor_test.go — Verifies persisted noise-state recovery reaches System Doctor.
package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func TestRecoveryDoctorChecks(t *testing.T) {
	t.Parallel()

	if checks := recoveryDoctorChecks(nil); len(checks) != 0 {
		t.Fatalf("healthy noise config checks = %#v, want none", checks)
	}

	collector := statediag.NewCollector()
	collector.Report(statediag.Diagnostic{
		Name: "noise_rule_state", Detail: "Defaults active.", Fix: "Reset rules.",
	})
	checks := recoveryDoctorChecks(collector)
	if len(checks) != 1 {
		t.Fatalf("checks = %#v, want one", checks)
	}
	if checks[0].Name != "noise_rule_state" || checks[0].Status != "warn" || checks[0].Fix == "" {
		t.Fatalf("unexpected Doctor check: %#v", checks[0])
	}
}
