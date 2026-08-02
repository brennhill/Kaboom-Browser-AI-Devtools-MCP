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
		Name: "noise_rule_state", CorrelationID: "noise_123", Detail: "Defaults active.", Fix: "Reset rules.",
	})
	checks := recoveryDoctorChecks(collector)
	if len(checks) != 1 {
		t.Fatalf("checks = %#v, want one", checks)
	}
	if checks[0].Name != "noise_rule_state" || checks[0].Status != "warn" || checks[0].Fix == "" {
		t.Fatalf("unexpected Doctor check: %#v", checks[0])
	}
	if checks[0].CorrelationID != "noise_123" {
		t.Fatalf("Doctor correlation = %q, want noise_123", checks[0].CorrelationID)
	}

	collector.Resolve("noise_rule_state")
	checks = recoveryDoctorChecks(collector)
	if checks[0].Status != "pass" || checks[0].Lifecycle != "recovered" || checks[0].RecoveredAt == "" {
		t.Fatalf("recovered Doctor check: %#v", checks[0])
	}
	if len(checks[0].History) != 2 {
		t.Fatalf("history = %#v, want active and recovered transitions", checks[0].History)
	}
}
