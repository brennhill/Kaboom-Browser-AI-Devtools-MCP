// census_test.go — Proves the census names what is running in terms an operator
// can act on, and never claims an empty machine is a healthy one.

package reaper_test

import (
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reaper"
)

func TestFormatCensusNamesEachInstance(t *testing.T) {
	now := time.Now()
	records := []instancereg.Record{
		{
			PID: 4285, Role: instancereg.RoleDaemon, Ports: []int{7890, 7891},
			Version: "0.9.0", StateDir: "/Users/dev/.kaboom",
			StartedAt:   now.Add(-61 * time.Hour).UTC().Format(time.RFC3339Nano),
			HeartbeatAt: now.Add(-3 * time.Second).UTC().Format(time.RFC3339Nano),
		},
		{
			PID: 65282, Role: instancereg.RoleBridge, Ports: []int{7890}, Version: "0.9.0",
			StartedAt:   now.Add(-31 * time.Hour).UTC().Format(time.RFC3339Nano),
			HeartbeatAt: now.Add(-2 * time.Second).UTC().Format(time.RFC3339Nano),
		},
	}
	out := reaper.FormatCensus(records, now)
	for _, want := range []string{"4285", "daemon", "7890", "7891", "0.9.0", "65282", "bridge"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatCensus() output is missing %q\n%s", want, out)
		}
	}
}

func TestFormatCensusMarksStaleHeartbeats(t *testing.T) {
	now := time.Now()
	records := []instancereg.Record{{
		PID: 999, Role: instancereg.RoleDaemon, Ports: []int{7890},
		StartedAt:   now.Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano),
		HeartbeatAt: now.Add(-45 * time.Minute).UTC().Format(time.RFC3339Nano),
	}}
	out := reaper.FormatCensus(records, now)
	if !strings.Contains(strings.ToLower(out), "stale") {
		t.Errorf("FormatCensus() did not flag a 45-minute-old heartbeat as stale\n%s", out)
	}
}

func TestFormatCensusSaysSoWhenEmpty(t *testing.T) {
	out := reaper.FormatCensus(nil, time.Now())
	if strings.TrimSpace(out) == "" {
		t.Error("FormatCensus() returned nothing for an empty machine")
	}
	if !strings.Contains(strings.ToLower(out), "no ") {
		t.Errorf("FormatCensus() did not state that nothing is running\n%s", out)
	}
}
