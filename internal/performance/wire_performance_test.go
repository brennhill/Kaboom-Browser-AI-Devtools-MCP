// wire_performance_test.go — Tests performance snapshot conversion and JSON contracts.
// Docs: docs/features/feature/performance-audit/index.md

package performance

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// JSON deserialization: camelCase → PerformanceSnapshot
// ============================================
// The extension sends camelCase JSON. These tests verify that Go's
// json.Unmarshal populates all fields correctly. If a JSON tag is
// changed (e.g. from "first_contentful_paint" to something else),
// these tests will fail.

func TestSnapshotJSON_AllWebVitalsDeserialize(t *testing.T) {
	t.Parallel()

	// This is the exact JSON shape the extension sends via POST /performance-snapshots
	jsonPayload := `{
		"url": "/dashboard",
		"timestamp": "2024-01-01T00:00:00Z",
		"timing": {
			"dom_content_loaded": 850,
			"load": 1500,
			"first_contentful_paint": 920,
			"largest_contentful_paint": 2400,
			"interaction_to_next_paint": 180,
			"time_to_first_byte": 110,
			"dom_interactive": 700
		},
		"network": {
			"request_count": 42,
			"transfer_size": 524288,
			"decoded_size": 1048576,
			"by_type": {"script": {"count": 5, "size": 200000}},
			"slowest_requests": [{"url": "/api/data", "duration": 350, "size": 8192}]
		},
		"long_tasks": {
			"count": 3,
			"total_blocking_time": 180,
			"longest": 95
		},
		"cumulative_layout_shift": 0.08
	}`

	var snap PerformanceSnapshot
	if err := json.Unmarshal([]byte(jsonPayload), &snap); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Assert every field deserialized — these fail if JSON tags are wrong
	if snap.URL != "/dashboard" {
		t.Errorf("URL = %q, want /dashboard", snap.URL)
	}
	if snap.Timing.DomContentLoaded != 850 {
		t.Errorf("DomContentLoaded = %v, want 850", snap.Timing.DomContentLoaded)
	}
	if snap.Timing.Load != 1500 {
		t.Errorf("Load = %v, want 1500", snap.Timing.Load)
	}
	if snap.Timing.FirstContentfulPaint == nil || *snap.Timing.FirstContentfulPaint != 920 {
		t.Errorf("FCP = %v, want 920", snap.Timing.FirstContentfulPaint)
	}
	if snap.Timing.LargestContentfulPaint == nil || *snap.Timing.LargestContentfulPaint != 2400 {
		t.Errorf("LCP = %v, want 2400", snap.Timing.LargestContentfulPaint)
	}
	if snap.Timing.InteractionToNextPaint == nil || *snap.Timing.InteractionToNextPaint != 180 {
		t.Errorf("INP = %v, want 180", snap.Timing.InteractionToNextPaint)
	}
	if snap.Timing.TimeToFirstByte != 110 {
		t.Errorf("TTFB = %v, want 110", snap.Timing.TimeToFirstByte)
	}
	if snap.Timing.DomInteractive != 700 {
		t.Errorf("DomInteractive = %v, want 700", snap.Timing.DomInteractive)
	}
	if snap.CLS == nil || *snap.CLS != 0.08 {
		t.Errorf("CLS = %v, want 0.08", snap.CLS)
	}
	if snap.Network.RequestCount != 42 {
		t.Errorf("RequestCount = %d, want 42", snap.Network.RequestCount)
	}
	if snap.Network.TransferSize != 524288 {
		t.Errorf("TransferSize = %d, want 524288", snap.Network.TransferSize)
	}
	if snap.LongTasks.Count != 3 {
		t.Errorf("LongTasks.Count = %d, want 3", snap.LongTasks.Count)
	}
	if snap.LongTasks.TotalBlockingTime != 180 {
		t.Errorf("TotalBlockingTime = %v, want 180", snap.LongTasks.TotalBlockingTime)
	}
}

func TestSnapshotJSON_UserTimingRoundTrip(t *testing.T) {
	t.Parallel()

	jsonPayload := `{
		"url": "/app",
		"timestamp": "2024-01-01T00:00:00Z",
		"timing": {"dom_content_loaded": 500, "load": 1000, "time_to_first_byte": 80, "dom_interactive": 400},
		"network": {"request_count": 10, "transfer_size": 50000, "decoded_size": 100000},
		"long_tasks": {"count": 0, "total_blocking_time": 0, "longest": 0},
		"cumulative_layout_shift": 0.01,
		"user_timing": {
			"marks": [
				{"name": "app-init", "start_time": 150},
				{"name": "hydration-complete", "start_time": 800}
			],
			"measures": [
				{"name": "hydration", "start_time": 150, "duration": 650}
			]
		}
	}`

	var snap PerformanceSnapshot
	if err := json.Unmarshal([]byte(jsonPayload), &snap); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// UserTiming must be populated — fails if field or tag is missing
	if snap.UserTiming == nil {
		t.Fatal("UserTiming is nil — user_timing JSON field not deserialized")
	}
	if len(snap.UserTiming.Marks) != 2 {
		t.Fatalf("UserTiming.Marks len = %d, want 2", len(snap.UserTiming.Marks))
	}
	if snap.UserTiming.Marks[0].Name != "app-init" {
		t.Errorf("Mark[0].Name = %q, want app-init", snap.UserTiming.Marks[0].Name)
	}
	if snap.UserTiming.Marks[0].StartTime != 150 {
		t.Errorf("Mark[0].StartTime = %v, want 150", snap.UserTiming.Marks[0].StartTime)
	}
	if snap.UserTiming.Marks[1].Name != "hydration-complete" {
		t.Errorf("Mark[1].Name = %q, want hydration-complete", snap.UserTiming.Marks[1].Name)
	}
	if len(snap.UserTiming.Measures) != 1 {
		t.Fatalf("UserTiming.Measures len = %d, want 1", len(snap.UserTiming.Measures))
	}
	if snap.UserTiming.Measures[0].Name != "hydration" {
		t.Errorf("Measure[0].Name = %q, want hydration", snap.UserTiming.Measures[0].Name)
	}
	if snap.UserTiming.Measures[0].Duration != 650 {
		t.Errorf("Measure[0].Duration = %v, want 650", snap.UserTiming.Measures[0].Duration)
	}

	// Round-trip: marshal back and verify JSON contains user_timing
	out, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, `"user_timing"`) {
		t.Error("Marshaled JSON missing user_timing field")
	}
	if !strings.Contains(outStr, `"app-init"`) {
		t.Error("Marshaled JSON missing mark name 'app-init'")
	}
	if !strings.Contains(outStr, `"hydration"`) {
		t.Error("Marshaled JSON missing measure name 'hydration'")
	}
}

func TestSnapshotJSON_UserTimingOmittedWhenAbsent(t *testing.T) {
	t.Parallel()

	jsonPayload := `{
		"url": "/simple",
		"timestamp": "2024-01-01T00:00:00Z",
		"timing": {"dom_content_loaded": 500, "load": 1000, "time_to_first_byte": 80, "dom_interactive": 400},
		"network": {"request_count": 5, "transfer_size": 25000, "decoded_size": 50000},
		"long_tasks": {"count": 0, "total_blocking_time": 0, "longest": 0}
	}`

	var snap PerformanceSnapshot
	if err := json.Unmarshal([]byte(jsonPayload), &snap); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if snap.UserTiming != nil {
		t.Error("UserTiming should be nil when absent from JSON")
	}

	// Marshal and verify user_timing is omitted (not null)
	out, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if strings.Contains(string(out), "user_timing") {
		t.Error("user_timing should be omitted from JSON when nil (omitempty)")
	}
}

// ============================================
// Full Web Vitals perf_diff: FCP + TTFB + CLS ratings
// ============================================
