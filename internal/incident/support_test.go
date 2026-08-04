// support_test.go — Privacy and determinism tests for Doctor support bundles.
package incident

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSupportBundleExcludesLocalIncidentData(t *testing.T) {
	store := NewStore(2)
	_, _ = store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: "https://private.example/path", Generation: 9, Evidence: LocalEvidence{Detail: "password=private"}})
	bundle := BuildSupportBundle("0.9.0", "darwin-arm64", store.DoctorSnapshot())
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"private.example", "password", "correlation", "local_detail", "generation"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("support bundle leaked %q: %s", forbidden, text)
		}
	}
	if bundle.SchemaVersion != 1 || len(bundle.Incidents) != 1 || bundle.Incidents[0].Fingerprint == "" {
		t.Fatalf("bundle = %#v", bundle)
	}
}

func TestSupportBundleTokenIsStableAndSnapshotSensitive(t *testing.T) {
	views := []DoctorView{{Code: CodeQueueSaturated, Fingerprint: "0123456789abcdef", State: StateDetected, Attempts: 1}}
	first := BuildSupportBundle("0.9.0", "linux-amd64", views)
	second := BuildSupportBundle("0.9.0", "linux-amd64", views)
	firstToken, err := SupportBundleToken(first)
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := SupportBundleToken(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstToken != secondToken {
		t.Fatal("identical bundles produced different tokens")
	}
	second.Incidents[0].Attempts++
	changedToken, err := SupportBundleToken(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstToken == changedToken {
		t.Fatal("changed bundle retained confirmation token")
	}
}
