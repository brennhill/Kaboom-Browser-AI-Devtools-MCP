// noise_doctor_test.go — Verifies persisted noise-state recovery reaches System Doctor.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func TestIncidentDoctorChecksProjectCanonicalLifecycle(t *testing.T) {
	t.Parallel()
	store := incident.NewStore(2)
	key, err := store.Detect(incident.Report{Code: incident.CodeStateRecoveryFailed, CorrelationID: "install_identity", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	store.Retry(key, 1, 1)
	store.Exhaust(key, 1)
	checks := incidentDoctorChecks(store)
	if len(checks) != 1 || checks[0].Name != string(incident.CodeStateRecoveryFailed) || checks[0].Status != "fail" {
		t.Fatalf("incident Doctor checks = %#v", checks)
	}
	if checks[0].CorrelationID != "install_identity" || checks[0].RecoveryAttempt != 1 || checks[0].RecoveryOutcome != "exhausted" || len(checks[0].History) != 3 {
		t.Fatalf("incident Doctor lifecycle = %#v", checks[0])
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(checks[0].Fingerprint) || checks[0].Fingerprint == "install_identity" {
		t.Fatalf("incident Doctor fingerprint = %q", checks[0].Fingerprint)
	}
	if checks[0].RecoveredAt != "" || checks[0].History[0].Outcome != "pending" || checks[0].History[1].Outcome != "pending" || checks[0].History[2].Outcome != "exhausted" {
		t.Fatalf("incident Doctor transition outcomes = %#v", checks[0])
	}
}

func TestIncidentDoctorChecksTreatFatalDetectionAsFailure(t *testing.T) {
	t.Parallel()
	store := incident.NewStore(1)
	if _, err := store.Detect(incident.Report{Code: incident.CodeDaemonRestartLoop, CorrelationID: "restart", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	check := incidentDoctorChecks(store)[0]
	if check.Status != "fail" || check.RecoveryOutcome != "pending" {
		t.Fatalf("fatal detected Doctor check = %#v", check)
	}
}

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

func TestRecoveryDoctorChecksExposeIncidentTimelineFields(t *testing.T) {
	collector := statediag.NewCollector()
	deadline := time.Now().UTC().Add(time.Minute)
	collector.Report(statediag.Diagnostic{
		Name: "content_readiness", CorrelationID: "nav-123", Detail: "waiting", Fix: "retry",
		ExpectedNextTransition: "content_ready", Deadline: deadline,
	})
	check := recoveryDoctorChecks(collector)[0]
	if check.ExpectedNextTransition != "content_ready" || check.Deadline == "" || check.RecoveryAttempt != 1 ||
		check.RecoveryOutcome != "pending" || len(check.History) != 1 || check.History[0].Event != "failure_detected" {
		t.Fatalf("Doctor incident timeline = %#v", check)
	}
	collector.Resolve("content_readiness")
	check = recoveryDoctorChecks(collector)[0]
	if check.Status != "pass" || check.LastSuccessfulTransition != "state_verified" || check.RecoveryOutcome != "recovered" {
		t.Fatalf("Doctor recovered timeline = %#v", check)
	}
}

func TestRecoveryDoctorChecksExposeRecoveredIncidentDrops(t *testing.T) {
	collector := statediag.NewCollector()
	for index := 0; index < 1_000; index++ {
		name := fmt.Sprintf("incident_%03d", index)
		collector.Report(statediag.Diagnostic{Name: name, Detail: "safe", Fix: "retry"})
		collector.Resolve(name)
	}

	checks := recoveryDoctorChecks(collector)
	retention := checks[len(checks)-1]
	if len(checks) != 101 {
		t.Fatalf("Doctor check count = %d, want 100 retained incidents plus retention summary", len(checks))
	}
	if retention.Name != "state_recovery_retention" || retention.Status != "pass" || retention.Occurrences != 900 {
		t.Fatalf("retention check = %#v", retention)
	}
	payload, err := json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 100_000 {
		t.Fatalf("Doctor recovery payload = %d bytes, want bounded below 100000", len(payload))
	}
}
