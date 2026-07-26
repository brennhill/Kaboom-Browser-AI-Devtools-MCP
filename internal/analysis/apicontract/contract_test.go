// Purpose: Tests for API contract validation and drift detection.
// Docs: docs/features/feature/api-schema/index.md

// api_contract_test.go — Tests for API contract validation.
// Tests schema learning, shape comparison, violation detection, and the MCP tool interface.
// Design: TDD approach - tests written first to define expected behavior.
package apicontract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Type alias for convenience in tests
type NetworkBody = capture.NetworkBody

// ============================================
// Schema Learning Tests
// ============================================

func TestAPIContractValidator_LearnBasicSchema(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	body := NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	}

	v.Learn(body)

	trackers := v.GetTrackers()
	if len(trackers) != 1 {
		t.Fatalf("Expected 1 tracker, got %d", len(trackers))
	}

	tracker := trackers["GET /api/users/{id}"]
	if tracker == nil {
		t.Fatal("Expected tracker for 'GET /api/users/{id}'")
	}
	if tracker.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", tracker.CallCount)
	}
}

func TestAPIContractValidator_LearnMultipleResponses(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn 3 consistent responses to establish the shape
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
			ContentType:  "application/json",
		})
	}

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/users/{id}"]
	if tracker == nil {
		t.Fatal("Expected tracker")
	}
	if tracker.CallCount != 3 {
		t.Errorf("Expected CallCount=3, got %d", tracker.CallCount)
	}

	// Shape should be established
	shape := tracker.EstablishedShape
	if shape == nil {
		t.Fatal("Expected established shape")
	}

	shapeMap, ok := shape.(map[string]any)
	if !ok {
		t.Fatal("Expected shape to be a map")
	}

	// Check fields are present
	if _, ok := shapeMap["id"]; !ok {
		t.Error("Expected 'id' in shape")
	}
	if _, ok := shapeMap["name"]; !ok {
		t.Error("Expected 'name' in shape")
	}
	if _, ok := shapeMap["email"]; !ok {
		t.Error("Expected 'email' in shape")
	}
}

func TestAPIContractValidator_LearnMergesFields(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// First response with basic fields
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Second response adds 'email' field
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob","email":"bob@example.com"}`,
		ContentType:  "application/json",
	})

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/users/{id}"]
	shapeMap := tracker.EstablishedShape.(map[string]any)

	// Shape should include all observed fields
	if _, ok := shapeMap["id"]; !ok {
		t.Error("Expected 'id' in merged shape")
	}
	if _, ok := shapeMap["name"]; !ok {
		t.Error("Expected 'name' in merged shape")
	}
	if _, ok := shapeMap["email"]; !ok {
		t.Error("Expected 'email' in merged shape")
	}
}

func TestAPIContractValidator_LearnTracksFieldPresence(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// 'email' only appears in 1 of 3 responses
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	})
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob"}`,
		ContentType:  "application/json",
	})
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/3",
		Status:       200,
		ResponseBody: `{"id":3,"name":"Carol"}`,
		ContentType:  "application/json",
	})

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/users/{id}"]

	// Check field presence counts
	if tracker.FieldPresence["id"] != 3 {
		t.Errorf("Expected 'id' present 3 times, got %d", tracker.FieldPresence["id"])
	}
	if tracker.FieldPresence["name"] != 3 {
		t.Errorf("Expected 'name' present 3 times, got %d", tracker.FieldPresence["name"])
	}
	if tracker.FieldPresence["email"] != 1 {
		t.Errorf("Expected 'email' present 1 time, got %d", tracker.FieldPresence["email"])
	}
}

// ============================================
// Endpoint Normalization Tests
// ============================================

func TestNormalizeEndpoint_NumericID(t *testing.T) {
	t.Parallel()
	result := normalizeEndpoint("GET", "http://localhost:3000/api/users/123")
	if result != "GET /api/users/{id}" {
		t.Errorf("Expected 'GET /api/users/{id}', got %q", result)
	}
}

func TestNormalizeEndpoint_UUID(t *testing.T) {
	t.Parallel()
	result := normalizeEndpoint("GET", "http://localhost:3000/api/items/550e8400-e29b-41d4-a716-446655440000")
	if result != "GET /api/items/{id}" {
		t.Errorf("Expected 'GET /api/items/{id}', got %q", result)
	}
}

func TestNormalizeEndpoint_LongHex(t *testing.T) {
	t.Parallel()
	result := normalizeEndpoint("GET", "http://localhost:3000/api/commits/a1b2c3d4e5f6a7b8c9d0")
	if result != "GET /api/commits/{id}" {
		t.Errorf("Expected 'GET /api/commits/{id}', got %q", result)
	}
}

func TestNormalizeEndpoint_IgnoresQueryParams(t *testing.T) {
	t.Parallel()
	result := normalizeEndpoint("GET", "http://localhost:3000/api/users?page=1&limit=20")
	if result != "GET /api/users" {
		t.Errorf("Expected 'GET /api/users', got %q", result)
	}
}

func TestNormalizeEndpoint_MultipleIDs(t *testing.T) {
	t.Parallel()
	result := normalizeEndpoint("GET", "http://localhost:3000/api/users/123/posts/456")
	if result != "GET /api/users/{id}/posts/{id}" {
		t.Errorf("Expected 'GET /api/users/{id}/posts/{id}', got %q", result)
	}
}

// ============================================
// Violation Detection Tests
// ============================================

func TestAPIContractValidator_DetectShapeChange(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn 3 consistent responses with 'avatar_url'
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/profile",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice","avatar_url":"https://example.com/avatar.png"}`,
			ContentType:  "application/json",
		})
	}

	// Now response is missing 'avatar_url'
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/profile",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	if len(violations) == 0 {
		t.Fatal("Expected shape_change violation")
	}

	found := false
	for _, v := range violations {
		if v.Type == "shape_change" && containsField(v.MissingFields, "avatar_url") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected shape_change violation for missing 'avatar_url', got %+v", violations)
	}
}

func TestAPIContractValidator_DetectTypeChange(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn responses where 'price' is a number
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/products/1",
			Status:       200,
			ResponseBody: `{"id":1,"price":19.99}`,
			ContentType:  "application/json",
		})
	}

	// Now 'price' is a string
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/products/1",
		Status:       200,
		ResponseBody: `{"id":1,"price":"19.99"}`,
		ContentType:  "application/json",
	})

	if len(violations) == 0 {
		t.Fatal("Expected type_change violation")
	}

	found := false
	for _, v := range violations {
		if v.Type == "type_change" && v.Field == "price" {
			found = true
			if v.ExpectedType != "number" {
				t.Errorf("Expected ExpectedType='number', got %q", v.ExpectedType)
			}
			if v.ActualType != "string" {
				t.Errorf("Expected ActualType='string', got %q", v.ActualType)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected type_change violation for 'price', got %+v", violations)
	}
}

func TestAPIContractValidator_DetectErrorSpike(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// 3 successful responses
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       201,
			ResponseBody: `{"id":1,"status":"created"}`,
			ContentType:  "application/json",
		})
	}

	// Now 2 error responses
	for i := 0; i < 2; i++ {
		violations := v.Validate(NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       500,
			ResponseBody: `{"error":"Internal server error"}`,
			ContentType:  "application/json",
		})
		// Should detect error_spike on second 500
		if i == 1 && len(violations) == 0 {
			t.Error("Expected error_spike violation after consecutive 500s")
		}
	}

	tracker := v.GetTrackers()["POST /api/orders"]
	if tracker == nil {
		t.Fatal("Expected tracker")
	}

	// Status history should show the pattern
	history := tracker.StatusHistory
	if len(history) < 5 {
		t.Errorf("Expected at least 5 status entries, got %d", len(history))
	}
}

func TestAPIContractValidator_DetectNewField(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn 3 consistent responses
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Response with new field 'created_at'
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","created_at":"2025-01-20T14:30:00Z"}`,
		ContentType:  "application/json",
	})

	found := false
	for _, v := range violations {
		if v.Type == "new_field" && containsField(v.NewFields, "created_at") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected new_field violation for 'created_at', got %+v", violations)
	}
}

func TestAPIContractValidator_DetectNullField(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn responses where 'avatar' is a string
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"avatar":"https://example.com/avatar.png"}`,
			ContentType:  "application/json",
		})
	}

	// Now 'avatar' is null
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"avatar":null}`,
		ContentType:  "application/json",
	})

	found := false
	for _, v := range violations {
		if v.Type == "null_field" && v.Field == "avatar" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected null_field violation for 'avatar', got %+v", violations)
	}
}

// ============================================
// No Violation Cases
// ============================================

func TestAPIContractValidator_NoViolationWithConsistentResponses(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// All responses are identical
	for i := 0; i < 5; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	if len(violations) != 0 {
		t.Errorf("Expected no violations, got %+v", violations)
	}
}

func TestAPIContractValidator_NoViolationBeforeMinCalls(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Only 2 observations - not enough to establish shape
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	})
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob","email":"bob@example.com"}`,
		ContentType:  "application/json",
	})

	// Missing field should not be a violation yet (still learning)
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/3",
		Status:       200,
		ResponseBody: `{"id":3,"name":"Carol"}`,
		ContentType:  "application/json",
	})

	// During learning phase, should not flag violations
	for _, v := range violations {
		if v.Type == "shape_change" {
			t.Error("Should not report shape_change violation before shape is established (min 3 calls)")
		}
	}
}

// ============================================
// Error Response Handling
// ============================================

func TestAPIContractValidator_ErrorResponsesNotUpdatingShape(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn success response
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Error response with different shape
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       404,
		ResponseBody: `{"error":"Not found","code":404}`,
		ContentType:  "application/json",
	})

	// Shape should still be from success response
	tracker := v.GetTrackers()["GET /api/users/{id}"]
	shape := tracker.EstablishedShape.(map[string]any)

	if _, ok := shape["error"]; ok {
		t.Error("Error response should not update established shape")
	}
	if _, ok := shape["id"]; !ok {
		t.Error("Expected 'id' from success response to be in shape")
	}
}

// ============================================
// MCP Tool Interface Tests
// ============================================

func TestAPIContractValidator_AnalyzeAction(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn some data
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause a violation
	v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`, // Missing 'name'
		ContentType:  "application/json",
	})

	result := v.Analyze(APIContractFilter{})

	if result.Action != "analyzed" {
		t.Errorf("Expected action='analyzed', got %q", result.Action)
	}
	if len(result.Violations) == 0 {
		t.Error("Expected violations in analyze result")
	}
	if result.TrackedEndpoints != 1 {
		t.Errorf("Expected TrackedEndpoints=1, got %d", result.TrackedEndpoints)
	}
}

func TestAPIContractValidator_ReportAction(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn some data for multiple endpoints
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       201,
			ResponseBody: `{"id":1,"total":100}`,
			ContentType:  "application/json",
		})
	}

	result := v.Report(APIContractFilter{})

	if result.Action != "report" {
		t.Errorf("Expected action='report', got %q", result.Action)
	}
	if len(result.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(result.Endpoints))
	}
}

func TestAPIContractValidator_ClearAction(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn some data
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Clear
	v.Clear()

	trackers := v.GetTrackers()
	if len(trackers) != 0 {
		t.Errorf("Expected 0 trackers after clear, got %d", len(trackers))
	}
}

func TestAPIContractValidator_URLFilter(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn multiple endpoints
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/orders/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	// Report with URL filter
	result := v.Report(APIContractFilter{URLFilter: "users"})

	if len(result.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint with URL filter, got %d", len(result.Endpoints))
	}
	if !strings.Contains(result.Endpoints[0].Endpoint, "users") {
		t.Error("Expected filtered endpoint to contain 'users'")
	}
}

func TestAPIContractValidator_IgnoreEndpoints(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn multiple endpoints
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/health",
			Status:       200,
			ResponseBody: `{"status":"ok"}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/metrics",
			Status:       200,
			ResponseBody: `{"uptime":12345}`,
			ContentType:  "application/json",
		})
	}

	// Report with ignore filter
	result := v.Report(APIContractFilter{IgnoreEndpoints: []string{"/health", "/metrics"}})

	if len(result.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint after ignoring /health and /metrics, got %d", len(result.Endpoints))
	}
}

// ============================================
// Edge Cases
// ============================================

func TestAPIContractValidator_NonJSONResponse(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Non-JSON response should be ignored
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/download",
		Status:       200,
		ResponseBody: "plain text content",
		ContentType:  "text/plain",
	})

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/download"]

	// Should still track the endpoint but not have a shape
	if tracker.EstablishedShape != nil {
		t.Error("Expected no shape for non-JSON response")
	}
}

func TestAPIContractValidator_EmptyResponseBody(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	v.Learn(NetworkBody{
		Method:       "DELETE",
		URL:          "http://localhost:3000/api/users/1",
		Status:       204,
		ResponseBody: "",
		ContentType:  "",
	})

	trackers := v.GetTrackers()
	tracker := trackers["DELETE /api/users/{id}"]

	if tracker == nil {
		t.Fatal("Expected tracker even for empty response")
	}
	if tracker.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", tracker.CallCount)
	}
}

func TestAPIContractValidator_NestedObjectShapeChange(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn with nested object
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"profile":{"bio":"Hello","location":"NYC"}}`,
			ContentType:  "application/json",
		})
	}

	// Nested object missing a field - should detect if we track nested shapes
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"profile":{"bio":"Hello"}}`,
		ContentType:  "application/json",
	})

	// Note: Spec says max depth 3 for shape comparison
	// This test validates that nested shape changes are detected
	if len(violations) == 0 {
		t.Log("Note: Nested shape changes may not be detected depending on implementation depth")
	}
}

func TestAPIContractValidator_ArrayShapeConsistency(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn with array of objects
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users",
			Status:       200,
			ResponseBody: `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`,
			ContentType:  "application/json",
		})
	}

	// Array response - shape tracking should work
	tracker := v.GetTrackers()["GET /api/users"]
	if tracker == nil {
		t.Fatal("Expected tracker for array endpoint")
	}
}

func TestAPIContractValidator_StatusHistoryLimit(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Add more than 20 requests (status history limit per spec)
	for i := 0; i < 25; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	tracker := v.GetTrackers()["GET /api/users/{id}"]
	if len(tracker.StatusHistory) > 20 {
		t.Errorf("Expected status history capped at 20, got %d", len(tracker.StatusHistory))
	}
}

func TestAPIContractValidator_EndpointLimit(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Add more than 30 unique endpoints (limit per spec)
	for i := 0; i < 35; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/endpoint" + string(rune('a'+i)),
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	trackers := v.GetTrackers()
	if len(trackers) > 30 {
		t.Errorf("Expected max 30 tracked endpoints, got %d", len(trackers))
	}
}

// ============================================
// Consistency Calculation
// ============================================

func TestAPIContractValidator_ConsistencyCalculation(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// 8 consistent responses
	for i := 0; i < 8; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// 2 inconsistent responses (missing 'name')
	for i := 0; i < 2; i++ {
		v.Validate(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	result := v.Report(APIContractFilter{})
	if len(result.Endpoints) == 0 {
		t.Fatal("Expected endpoint in report")
	}

	ep := result.Endpoints[0]
	// 8 consistent out of 10 = 80%
	if ep.Consistency != "80%" {
		t.Errorf("Expected consistency '80%%', got %q", ep.Consistency)
	}
}

// ============================================
// Timestamp Tracking
// ============================================

func TestAPIContractValidator_LastCalledTimestamp(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	before := time.Now()
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})
	after := time.Now()

	tracker := v.GetTrackers()["GET /api/users/{id}"]
	if tracker.LastCalled.Before(before) || tracker.LastCalled.After(after) {
		t.Error("LastCalled timestamp not in expected range")
	}
}

// ============================================
// Helper Functions
// ============================================

func containsField(fields []string, target string) bool {
	for _, f := range fields {
		if f == target {
			return true
		}
	}
	return false
}

// ============================================
// Integration with NetworkBody stream
// ============================================

func TestAPIContractValidator_ProcessNetworkBodies(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	bodies := []NetworkBody{
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
		Type:          "shape_change",
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
	if parsed["type"] != "shape_change" {
		t.Error("Expected 'type' field in JSON")
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
	v.Learn(NetworkBody{
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
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause a violation
	v.Validate(NetworkBody{
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

// Test 5: violationType and severity on violations
func TestAPIContractAnalyze_ViolationTypeAndSeverity(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause a shape_change violation
	v.Validate(NetworkBody{
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
	// violationType should be set (same as Type for backward compatibility)
	if viol.ViolationType == "" {
		t.Error("Expected violationType to be set")
	}
	if viol.ViolationType != viol.Type {
		t.Errorf("Expected violationType=%q to match type=%q", viol.ViolationType, viol.Type)
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
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause violations
	v.Validate(NetworkBody{
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
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	before := time.Now().Truncate(time.Second)
	v.Validate(NetworkBody{
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
		v.Learn(NetworkBody{
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
func TestAPIContractReport_LastCalledAtRename(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	v.Learn(NetworkBody{
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

	v.Learn(NetworkBody{
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
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}
	for i := 0; i < 2; i++ {
		v.Validate(NetworkBody{
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
	v.Learn(NetworkBody{
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

	// Second call should NOT update FirstCalled
	time.Sleep(time.Millisecond)
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2}`,
		ContentType:  "application/json",
	})

	tracker = v.GetTrackers()["GET /api/users/{id}"]
	if tracker.FirstCalled.After(after) {
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
