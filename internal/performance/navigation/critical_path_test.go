// critical_path_test.go — Tests deterministic navigation phase analysis.
// Docs: docs/features/feature/navigation-critical-path/index.md

package navigation

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func floatPointer(value float64) *float64 { return &value }

func TestBuildCriticalPathOrdersEvidenceAndMarksUnavailablePhases(t *testing.T) {
	snapshot := performance.PerformanceSnapshot{
		URL: "/dashboard",
		Timing: performance.PerformanceTiming{
			TimeToFirstByte: 80, FirstContentfulPaint: floatPointer(500), LargestContentfulPaint: floatPointer(900),
		},
		UserTiming: &performance.UserTimingData{
			Measures: []performance.UserTimingEntry{{Name: "store-update", StartTime: 330, Duration: 20}},
		},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{URL: "https://app.test/auth/token", StartTime: 90, Duration: 100, ResponseEnd: 190},
		{URL: "https://app.test/api/projects", StartTime: 200, Duration: 170, ResponseEnd: 370, QueueingMs: 15},
	}
	result := BuildCriticalPath(snapshot, waterfall)
	if result.DominantSegment != "first_contentful_paint" {
		t.Fatalf("dominant segment = %q", result.DominantSegment)
	}
	if result.Phases[1].Name != "authentication" || result.Phases[1].Status != "available" {
		t.Fatalf("authentication phase = %+v", result.Phases[1])
	}
	if result.Phases[3].Name != "state_update" || result.Phases[3].DurationMs == nil || *result.Phases[3].DurationMs != 20 {
		t.Fatalf("state phase = %+v", result.Phases[3])
	}
	if result.Phases[4].Name != "react_commit" || result.Phases[4].Status != "unavailable" || result.Phases[4].DurationMs != nil {
		t.Fatalf("React phase must distinguish unavailable from zero: %+v", result.Phases[4])
	}
	if result.Gaps[1].Status != "available" || result.Gaps[1].DurationMs == nil || *result.Gaps[1].DurationMs != 10 {
		t.Fatalf("auth-to-backend gap = %+v", result.Gaps[1])
	}
}

func TestBuildCriticalPathReportsMissingEvidenceHonestly(t *testing.T) {
	result := BuildCriticalPath(performance.PerformanceSnapshot{}, nil)
	if result.Status != "partial" || len(result.Gaps) == 0 || result.Gaps[0].Status != "unavailable" || result.Gaps[0].DurationMs != nil {
		t.Fatalf("missing evidence was not explicit: %+v", result)
	}
	for _, phase := range result.Phases {
		if phase.Status == "unavailable" && phase.DurationMs != nil {
			t.Fatalf("unavailable phase has synthetic duration: %+v", phase)
		}
	}
}
