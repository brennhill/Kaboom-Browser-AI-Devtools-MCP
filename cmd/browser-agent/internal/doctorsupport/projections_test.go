// projections_test.go — Verifies bounded Doctor lifecycle projections.

package doctorsupport

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func TestBoundedIntSaturatesUntrustedCounters(t *testing.T) {
	t.Parallel()
	if boundedInt(7) != 7 || boundedInt(math.MaxUint64) != math.MaxInt {
		t.Fatal("bounded diagnostic counter did not saturate")
	}
}

func TestIncidentChecksProjectCanonicalLifecycle(t *testing.T) {
	t.Parallel()
	store := incident.NewStore(2)
	key, err := store.Detect(incident.Report{Code: incident.CodeStateRecoveryFailed, CorrelationID: "install_identity", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	store.Retry(key, 1, 1)
	store.Exhaust(key, 1)
	checks := incidentChecks(store)
	if len(checks) != 1 || checks[0].Name != string(incident.CodeStateRecoveryFailed) || checks[0].Status != "fail" {
		t.Fatalf("incident checks = %#v", checks)
	}
	check := checks[0]
	if check.CorrelationID != "install_identity" || check.RecoveryAttempt != 1 || check.RecoveryOutcome != "exhausted" || len(check.History) != 3 {
		t.Fatalf("incident lifecycle = %#v", check)
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(check.Fingerprint) || check.Fingerprint == "install_identity" {
		t.Fatalf("incident fingerprint = %q", check.Fingerprint)
	}
	if check.RecoveredAt != "" || check.History[0].Outcome != "pending" || check.History[2].Outcome != "exhausted" {
		t.Fatalf("incident transitions = %#v", check)
	}
}

func TestIncidentChecksTreatFatalDetectionAsFailure(t *testing.T) {
	t.Parallel()
	store := incident.NewStore(1)
	if _, err := store.Detect(incident.Report{Code: incident.CodeDaemonRestartLoop, CorrelationID: "restart", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	check := incidentChecks(store)[0]
	if check.Status != "fail" || check.RecoveryOutcome != "pending" {
		t.Fatalf("fatal incident check = %#v", check)
	}
}

func TestRecoveryChecksProjectActiveAndRecoveredLifecycle(t *testing.T) {
	t.Parallel()
	if checks := recoveryChecks(nil); len(checks) != 0 {
		t.Fatalf("nil recovery checks = %#v", checks)
	}
	collector := statediag.NewCollector()
	deadline := time.Now().UTC().Add(time.Minute)
	collector.Report(statediag.Diagnostic{
		Name: "content_readiness", CorrelationID: "nav-123", Detail: "waiting", Fix: "retry",
		ExpectedNextTransition: "content_ready", Deadline: deadline,
	})
	check := recoveryChecks(collector)[0]
	if check.Status != "warn" || check.ExpectedNextTransition != "content_ready" || check.Deadline == "" || check.RecoveryAttempt != 1 || len(check.History) != 1 {
		t.Fatalf("active recovery check = %#v", check)
	}
	collector.Resolve("content_readiness")
	check = recoveryChecks(collector)[0]
	if check.Status != "pass" || check.Lifecycle != "recovered" || check.RecoveredAt == "" || check.LastSuccessfulTransition != "state_verified" || len(check.History) != 2 {
		t.Fatalf("recovered check = %#v", check)
	}
}

func TestRecoveryChecksExposeBoundedRetention(t *testing.T) {
	collector := statediag.NewCollector()
	for index := 0; index < 1_000; index++ {
		name := fmt.Sprintf("incident_%03d", index)
		collector.Report(statediag.Diagnostic{Name: name, Detail: "safe", Fix: "retry"})
		collector.Resolve(name)
	}
	checks := recoveryChecks(collector)
	retention := checks[len(checks)-1]
	if len(checks) != 101 || retention.Name != "state_recovery_retention" || retention.Status != "pass" || retention.Occurrences != 900 {
		t.Fatalf("retention projection = %#v (%d checks)", retention, len(checks))
	}
	payload, err := json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 100_000 {
		t.Fatalf("Doctor recovery payload = %d bytes", len(payload))
	}
}
