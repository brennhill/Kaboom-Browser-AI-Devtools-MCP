// noise_doctor_test.go — Verifies persisted noise-state recovery reaches System Doctor.
package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

func TestNoisePersistenceDoctorChecks(t *testing.T) {
	t.Parallel()

	if checks := noisePersistenceDoctorChecks(noise.NewNoiseConfig()); len(checks) != 0 {
		t.Fatalf("healthy noise config checks = %#v, want none", checks)
	}

	store, err := persistence.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("noise", "rules", []byte("{")); err != nil {
		t.Fatal(err)
	}
	config := noise.NewNoiseConfigWithStore(store)
	checks := noisePersistenceDoctorChecks(config)
	if len(checks) != 1 {
		t.Fatalf("checks = %#v, want one", checks)
	}
	if checks[0].Name != "noise_rule_state" || checks[0].Status != "warn" || checks[0].Fix == "" {
		t.Fatalf("unexpected Doctor check: %#v", checks[0])
	}
}
