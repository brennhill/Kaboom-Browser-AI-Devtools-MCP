// contract_analyze_test.go — Tests API contract analysis responses and violations.
// Docs: docs/features/feature/api-schema/index.md

// Tests schema learning, shape comparison, violation detection, and the MCP tool interface.
// Design: TDD approach - tests written first to define expected behavior.
package apicontract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestAPIContractValidator_ProcessNetworkBodies(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	bodies := []types.NetworkBody{
		{Method: "GET", URL: "http://localhost:3000/api/users/1", Status: 200, ResponseBody: `{"id":1,"name":"Alice"}`, ContentType: "application/json"},
		{Method: "GET", URL: "http://localhost:3000/api/users/2", Status: 200, ResponseBody: `{"id":2,"name":"Bob"}`, ContentType: "application/json"},
		{Method: "GET", URL: "http://localhost:3000/api/users/3", Status: 200, ResponseBody: `{"id":3,"name":"Carol"}`, ContentType: "application/json"},
		{Method: "POST", URL: "http://localhost:3000/api/users", Status: 201, ResponseBody: `{"id":4,"name":"Dave"}`, ContentType: "application/json"},
	}

	for _, body := range bodies {
		v.Learn(body)
	}

	trackers := v.GetTrackers()
	if len(trackers) != 2 {
		t.Errorf("Expected 2 unique endpoints, got %d", len(trackers))
	}
}

// ============================================
// JSON Serialization Tests
// ============================================

func TestAPIContractViolation_JSONSerialization(t *testing.T) {
	t.Parallel()
	violation := APIContractViolation{
		Endpoint:      "GET /api/users/{id}",
		ViolationType: "shape_change",
		Description:   "Field 'avatar_url' was present in 5 responses but missing in the latest",
		MissingFields: []string{"avatar_url"},
	}

	data, err := json.Marshal(violation)
	if err != nil {
		t.Fatalf("Failed to marshal violation: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal violation: %v", err)
	}

	if parsed["endpoint"] != "GET /api/users/{id}" {
		t.Error("Expected 'endpoint' field in JSON")
	}
	if parsed["violation_type"] != "shape_change" {
		t.Error("Expected 'violation_type' field in JSON")
	}
	if _, exists := parsed["type"]; exists {
		t.Error("Unexpected duplicate 'type' field in JSON")
	}
}

// ============================================
// Schema Improvement Tests (LLM-Optimized Responses)
// ============================================

// Test 1: analyzedAt timestamp in RFC3339 format
func TestAPIContractAnalyze_AnalyzedAtTimestamp(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	before := time.Now().Truncate(time.Second)
	result := v.Analyze(APIContractFilter{})
	after := time.Now().Add(time.Second).Truncate(time.Second)

	if result.AnalyzedAt == "" {
		t.Fatal("Expected analyzedAt to be set")
	}

	parsed, err := time.Parse(time.RFC3339, result.AnalyzedAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 format for analyzedAt, got %q: %v", result.AnalyzedAt, err)
	}

	if parsed.Before(before) || parsed.After(after) {
		t.Errorf("analyzedAt %v not in expected range [%v, %v]", parsed, before, after)
	}
}

// Test 2: dataWindowStartedAt timestamp (when data collection began)
func TestAPIContractAnalyze_DataWindowStartedAt(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// No data yet - should be empty string
	result := v.Analyze(APIContractFilter{})
	if result.DataWindowStartedAt != "" {
		t.Errorf("Expected empty dataWindowStartedAt when no data, got %q", result.DataWindowStartedAt)
	}

	// Add some data
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})

	result = v.Analyze(APIContractFilter{})
	if result.DataWindowStartedAt == "" {
		t.Fatal("Expected dataWindowStartedAt to be set after learning data")
	}

	_, err := time.Parse(time.RFC3339, result.DataWindowStartedAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 format for dataWindowStartedAt, got %q: %v", result.DataWindowStartedAt, err)
	}
}

// Test 3: appliedFilter echo
func TestAPIContractAnalyze_AppliedFilter(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// No filter
	result := v.Analyze(APIContractFilter{})
	if result.AppliedFilter != nil {
		t.Errorf("Expected nil appliedFilter when no filter, got %v", result.AppliedFilter)
	}

	// With URL filter
	result = v.Analyze(APIContractFilter{URLFilter: "users"})
	if result.AppliedFilter == nil {
		t.Fatal("Expected appliedFilter to be set when URL filter provided")
	}
	if result.AppliedFilter.URL != "users" {
		t.Errorf("Expected appliedFilter.url='users', got %q", result.AppliedFilter.URL)
	}

	// With ignore_endpoints filter
	result = v.Analyze(APIContractFilter{IgnoreEndpoints: []string{"/health"}})
	if result.AppliedFilter == nil {
		t.Fatal("Expected appliedFilter to be set when ignore filter provided")
	}
	if len(result.AppliedFilter.IgnoreEndpoints) != 1 || result.AppliedFilter.IgnoreEndpoints[0] != "/health" {
		t.Errorf("Expected appliedFilter.ignoreEndpoints=['/health'], got %v", result.AppliedFilter.IgnoreEndpoints)
	}
}

// Test 4: summary object with counts
func TestAPIContractAnalyze_SummaryObject(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn 3 consistent responses to establish the shape
	for i := 0; i < 3; i++ {
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause a violation
	v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`, // Missing 'name'
		ContentType:  "application/json",
	})

	result := v.Analyze(APIContractFilter{})

	if result.Summary == nil {
		t.Fatal("Expected summary object")
	}
	if result.Summary.Violations == 0 {
		t.Error("Expected non-zero violation count in summary")
	}
	if result.Summary.Endpoints != 1 {
		t.Errorf("Expected 1 endpoint in summary, got %d", result.Summary.Endpoints)
	}
	if result.Summary.TotalRequests == 0 {
		t.Error("Expected non-zero total requests in summary")
	}
	if result.Summary.CleanEndpoints < 0 {
		t.Error("Expected non-negative clean endpoints in summary")
	}
}

// Test 5: canonical violation type and severity on violations
func TestAPIContractAnalyze_ViolationTypeAndSeverity(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	for i := 0; i < 3; i++ {
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause a shape_change violation
	v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`, // Missing 'name'
		ContentType:  "application/json",
	})

	result := v.Analyze(APIContractFilter{})

	if len(result.Violations) == 0 {
		t.Fatal("Expected violations")
	}

	viol := result.Violations[0]
	// violationType should be set.
	if viol.ViolationType == "" {
		t.Error("Expected violationType to be set")
	}
	// severity should be set
	if viol.Severity == "" {
		t.Error("Expected severity to be set")
	}
	// severity should be one of: critical, high, medium, low
	validSeverities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
	if !validSeverities[viol.Severity] {
		t.Errorf("Expected severity to be one of critical/high/medium/low, got %q", viol.Severity)
	}
}

// Test 6: affectedCallCount on violations
func TestAPIContractAnalyze_AffectedCallCount(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	for i := 0; i < 3; i++ {
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause violations
	v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})

	result := v.Analyze(APIContractFilter{})

	if len(result.Violations) == 0 {
		t.Fatal("Expected violations")
	}

	for _, viol := range result.Violations {
		if viol.AffectedCallCount < 1 {
			t.Errorf("Expected affectedCallCount >= 1, got %d", viol.AffectedCallCount)
		}
	}
}

// Test 7: firstSeenAt and lastSeenAt timestamps on violations
func TestAPIContractAnalyze_ViolationTimestamps(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	for i := 0; i < 3; i++ {
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	before := time.Now().Truncate(time.Second)
	v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})
	after := time.Now().Add(time.Second).Truncate(time.Second)

	result := v.Analyze(APIContractFilter{})

	if len(result.Violations) == 0 {
		t.Fatal("Expected violations")
	}

	viol := result.Violations[0]
	if viol.FirstSeenAt == "" {
		t.Error("Expected firstSeenAt to be set")
	}
	if viol.LastSeenAt == "" {
		t.Error("Expected lastSeenAt to be set")
	}

	firstSeen, err := time.Parse(time.RFC3339, viol.FirstSeenAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 for firstSeenAt, got %q: %v", viol.FirstSeenAt, err)
	}
	lastSeen, err := time.Parse(time.RFC3339, viol.LastSeenAt)
	if err != nil {
		t.Fatalf("Expected RFC3339 for lastSeenAt, got %q: %v", viol.LastSeenAt, err)
	}

	if firstSeen.Before(before) || firstSeen.After(after) {
		t.Errorf("firstSeenAt %v not in expected range [%v, %v]", firstSeen, before, after)
	}
	if lastSeen.Before(firstSeen) {
		t.Errorf("lastSeenAt should be >= firstSeenAt")
	}
}

// Test 8: possibleViolationTypes metadata array
func TestAPIContractAnalyze_PossibleViolationTypes(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	result := v.Analyze(APIContractFilter{})

	if result.PossibleViolationTypes == nil {
		t.Fatal("Expected possibleViolationTypes metadata array")
	}

	expected := map[string]bool{
		"shape_change": true,
		"type_change":  true,
		"error_spike":  true,
		"new_field":    true,
		"null_field":   true,
	}
	for _, vt := range result.PossibleViolationTypes {
		if !expected[vt] {
			t.Errorf("Unexpected violation type in metadata: %q", vt)
		}
		delete(expected, vt)
	}
	if len(expected) > 0 {
		t.Errorf("Missing violation types in metadata: %v", expected)
	}
}

// Test 9: hint when no violations found
func TestAPIContractAnalyze_HintWhenNoViolations(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn consistent data
	for i := 0; i < 3; i++ {
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	result := v.Analyze(APIContractFilter{})

	if len(result.Violations) != 0 {
		t.Fatal("Expected no violations for consistent data")
	}
	if result.Hint == "" {
		t.Error("Expected hint when no violations found")
	}
	if !strings.Contains(result.Hint, "No violations") || !strings.Contains(result.Hint, "consistent") {
		t.Errorf("Expected hint to mention 'No violations' and 'consistent', got %q", result.Hint)
	}
}

// Test 10: lastCalled renamed to lastCalledAt in report
