// contract_report_test.go — Tests API contract report serialization and tracking.
// Docs: docs/features/feature/api-schema/index.md

// Tests schema learning, shape comparison, violation detection, and the MCP tool interface.
// Design: TDD approach - tests written first to define expected behavior.
package apicontract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestAPIContractReport_LastCalledAtRename(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})

	result := v.Report(APIContractFilter{})
	if len(result.Endpoints) == 0 {
		t.Fatal("Expected endpoints in report")
	}

	ep := result.Endpoints[0]

	// lastCalledAt should be set (renamed from lastCalled)
	if ep.LastCalledAt == "" {
		t.Error("Expected lastCalledAt to be set")
	}
	_, err := time.Parse(time.RFC3339, ep.LastCalledAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 for lastCalledAt, got %q: %v", ep.LastCalledAt, err)
	}

	// Verify JSON serialization uses last_called_at (not last_called)
	data, _ := json.Marshal(ep)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if _, ok := parsed["last_called_at"]; !ok {
		t.Error("Expected 'last_called_at' key in JSON output")
	}
	if _, ok := parsed["last_called"]; ok {
		t.Error("Old 'last_called' key should not be in JSON output")
	}
}

// Test 11: firstCalledAt added to report
func TestAPIContractReport_FirstCalledAt(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})

	result := v.Report(APIContractFilter{})
	if len(result.Endpoints) == 0 {
		t.Fatal("Expected endpoints in report")
	}

	ep := result.Endpoints[0]
	if ep.FirstCalledAt == "" {
		t.Error("Expected firstCalledAt to be set")
	}
	_, err := time.Parse(time.RFC3339, ep.FirstCalledAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 for firstCalledAt, got %q: %v", ep.FirstCalledAt, err)
	}
}

// Test 12: consistencyScore numeric 0-1
func TestAPIContractReport_ConsistencyScore(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// 8 consistent, then 2 inconsistent = 80% = 0.8
	for i := 0; i < 8; i++ {
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}
	for i := 0; i < 2; i++ {
		v.Validate(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`, // Missing 'name'
			ContentType:  "application/json",
		})
	}

	result := v.Report(APIContractFilter{})
	if len(result.Endpoints) == 0 {
		t.Fatal("Expected endpoints in report")
	}

	ep := result.Endpoints[0]
	if ep.ConsistencyScore < 0 || ep.ConsistencyScore > 1 {
		t.Errorf("Expected consistencyScore in [0,1], got %f", ep.ConsistencyScore)
	}
	// 8 consistent / 10 total = 0.8
	if ep.ConsistencyScore < 0.79 || ep.ConsistencyScore > 0.81 {
		t.Errorf("Expected consistencyScore ~0.8, got %f", ep.ConsistencyScore)
	}
}

// Test 13: consistencyLevels explanation metadata in report
func TestAPIContractReport_ConsistencyLevels(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	result := v.Report(APIContractFilter{})

	if result.ConsistencyLevels == nil {
		t.Fatal("Expected consistencyLevels metadata in report")
	}

	// Should explain the consistency score ranges
	if len(result.ConsistencyLevels) == 0 {
		t.Error("Expected non-empty consistencyLevels")
	}

	// Should contain keys mapping score ranges to descriptions
	data, _ := json.Marshal(result.ConsistencyLevels)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// Should have at least some standard level descriptions
	if len(parsed) == 0 {
		t.Error("Expected consistencyLevels to have entries")
	}
}

// Test: Report also has analyzedAt
func TestAPIContractReport_AnalyzedAtTimestamp(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	before := time.Now().Truncate(time.Second)
	result := v.Report(APIContractFilter{})
	after := time.Now().Add(time.Second).Truncate(time.Second)

	if result.AnalyzedAt == "" {
		t.Fatal("Expected analyzedAt to be set in report")
	}

	parsed, err := time.Parse(time.RFC3339, result.AnalyzedAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 format for analyzedAt in report, got %q: %v", result.AnalyzedAt, err)
	}

	if parsed.Before(before) || parsed.After(after) {
		t.Errorf("analyzedAt %v not in expected range [%v, %v]", parsed, before, after)
	}
}

// Test: Report also has appliedFilter
func TestAPIContractReport_AppliedFilter(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	result := v.Report(APIContractFilter{URLFilter: "users"})

	if result.AppliedFilter == nil {
		t.Fatal("Expected appliedFilter to be set in report")
	}
	if result.AppliedFilter.URL != "users" {
		t.Errorf("Expected appliedFilter.url='users', got %q", result.AppliedFilter.URL)
	}
}

// Test: EndpointTracker stores FirstCalled timestamp
func TestAPIContractValidator_FirstCalledTracking(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	before := time.Now()
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})
	after := time.Now()

	tracker := v.GetTrackers()["GET /api/users/{id}"]
	if tracker.FirstCalled.IsZero() {
		t.Error("Expected FirstCalled to be set")
	}
	if tracker.FirstCalled.Before(before) || tracker.FirstCalled.After(after) {
		t.Error("FirstCalled not in expected range")
	}
	firstCalled := tracker.FirstCalled

	// Second call should NOT update FirstCalled
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2}`,
		ContentType:  "application/json",
	})

	tracker = v.GetTrackers()["GET /api/users/{id}"]
	if !tracker.FirstCalled.Equal(firstCalled) {
		t.Error("FirstCalled should not be updated on subsequent calls")
	}
}

// Test: Violation severity mapping
func TestAPIContractViolation_SeverityMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		violationType    string
		expectedSeverity string
	}{
		{"shape_change", "high"},
		{"type_change", "high"},
		{"error_spike", "critical"},
		{"new_field", "low"},
		{"null_field", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.violationType, func(t *testing.T) {
			severity := violationSeverity(tt.violationType)
			if severity != tt.expectedSeverity {
				t.Errorf("Expected severity %q for type %q, got %q", tt.expectedSeverity, tt.violationType, severity)
			}
		})
	}
}
