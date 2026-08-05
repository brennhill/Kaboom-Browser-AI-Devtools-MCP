// loader_test.go — Defines deterministic extension-settings recovery contracts.
package settingscache

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func TestReadUsesCanonicalStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	data, err := read()
	if err != nil || data != nil {
		t.Fatalf("read absent cache = %q, %v", data, err)
	}
	path, err := state.SettingsFile()
	if err != nil {
		t.Fatalf("SettingsFile() error = %v", err)
	}
	want := filepath.Join(stateRoot, "settings", "extension-settings.json")
	if path != want {
		t.Fatalf("SettingsFile() = %q, want %q", path, want)
	}
}

func TestLoadRejectsMissingRequiredBoundaries(t *testing.T) {
	if err := Load(nil, statediag.NewCollector()); err == nil {
		t.Fatal("Load accepted a nil apply boundary")
	}
	if err := Load(func(bool, time.Time) {}, nil); err == nil {
		t.Fatal("Load accepted a nil Doctor reporter")
	}
}

func TestLoadAppliesOnlyFreshValidPilotState(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	collector := statediag.NewCollector()
	var applied bool
	var appliedAt time.Time
	load(now, func() ([]byte, error) {
		return []byte(`{"ai_web_pilot_enabled":true,"timestamp":"1970-01-01T00:01:39Z","ext_session_id":"session"}`), nil
	}, func(enabled bool, updatedAt time.Time) {
		applied, appliedAt = enabled, updatedAt
	}, collector)

	if !applied || !appliedAt.Equal(now.Add(-time.Second)) {
		t.Fatalf("applied = %t at %v", applied, appliedAt)
	}
	if got := collector.Snapshot(); len(got) != 0 {
		t.Fatalf("fresh settings diagnostics = %#v", got)
	}
}

func TestLoadFailsOpenWithRedactedDoctorEvidence(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	for _, test := range []struct {
		name string
		read func() ([]byte, error)
	}{
		{name: "read failure", read: func() ([]byte, error) { return nil, errors.New("private/path token=secret") }},
		{name: "malformed", read: func() ([]byte, error) { return []byte(`{"token":"private-value"`), nil }},
		{name: "missing timestamp", read: func() ([]byte, error) { return []byte(`{"ai_web_pilot_enabled":true}`), nil }},
		{name: "future timestamp", read: func() ([]byte, error) {
			return []byte(`{"ai_web_pilot_enabled":true,"timestamp":"1970-01-01T00:01:42Z"}`), nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			collector := statediag.NewCollector()
			applied := false
			load(now, test.read, func(bool, time.Time) { applied = true }, collector)
			if applied {
				t.Fatal("invalid settings mutated runtime state")
			}
			diagnostics := collector.Snapshot()
			if len(diagnostics) != 1 || diagnostics[0].Name != diagnosticName || diagnostics[0].Lifecycle != statediag.LifecycleActive {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			if containsPrivateEvidence(diagnostics[0].Detail) {
				t.Fatalf("diagnostic leaked persisted evidence: %#v", diagnostics[0])
			}
		})
	}
}

func TestLoadTreatsAbsenceAndStaleStateAsExpectedFallbacks(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	for _, read := range []func() ([]byte, error){
		func() ([]byte, error) { return nil, nil },
		func() ([]byte, error) {
			return []byte(`{"ai_web_pilot_enabled":true,"timestamp":"1970-01-01T00:01:00Z"}`), nil
		},
		func() ([]byte, error) {
			return []byte(`{"timestamp":"1970-01-01T00:01:39Z"}`), nil
		},
	} {
		collector := statediag.NewCollector()
		applied := false
		load(now, read, func(bool, time.Time) { applied = true }, collector)
		if applied || len(collector.Snapshot()) != 0 {
			t.Fatalf("expected fallback applied=%t diagnostics=%#v", applied, collector.Snapshot())
		}
	}
}

func containsPrivateEvidence(value string) bool {
	return strings.Contains(value, "private/path") || strings.Contains(value, "private-value") || strings.Contains(value, "secret")
}
