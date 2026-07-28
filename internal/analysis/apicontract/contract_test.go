// contract_test.go — Tests API schema learning, normalization, and violation detection.
// Docs: docs/features/feature/api-schema/index.md

// Tests schema learning, shape comparison, violation detection, and the MCP tool interface.
// Design: TDD approach - tests written first to define expected behavior.
package apicontract

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Schema Learning Tests
// ============================================

func TestAPIContractValidator_LearnBasicSchema(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	body := types.NetworkBody{
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

func TestAPIContractViolationWireUsesCanonicalTypeFieldOnly(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(APIContractViolation{
		Endpoint:      "GET /api/items",
		ViolationType: "shape_change",
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["violation_type"] != "shape_change" {
		t.Fatalf("violation_type = %v, want shape_change", decoded["violation_type"])
	}
	if _, exists := decoded["type"]; exists {
		t.Fatalf("duplicate compatibility field type must not be emitted: %s", payload)
	}
}

func TestAPIContractValidator_LearnMultipleResponses(t *testing.T) {
	t.Parallel()
	v := NewAPIContractValidator()

	// Learn 3 consistent responses to establish the shape
	for i := 0; i < 3; i++ {
		v.Learn(types.NetworkBody{
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
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Second response adds 'email' field
	v.Learn(types.NetworkBody{
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
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	})
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob"}`,
		ContentType:  "application/json",
	})
	v.Learn(types.NetworkBody{
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
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/profile",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice","avatar_url":"https://example.com/avatar.png"}`,
			ContentType:  "application/json",
		})
	}

	// Now response is missing 'avatar_url'
	violations := v.Validate(types.NetworkBody{
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
		if v.ViolationType == "shape_change" && containsField(v.MissingFields, "avatar_url") {
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
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/products/1",
			Status:       200,
			ResponseBody: `{"id":1,"price":19.99}`,
			ContentType:  "application/json",
		})
	}

	// Now 'price' is a string
	violations := v.Validate(types.NetworkBody{
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
		if v.ViolationType == "type_change" && v.Field == "price" {
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
		v.Learn(types.NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       201,
			ResponseBody: `{"id":1,"status":"created"}`,
			ContentType:  "application/json",
		})
	}

	// Now 2 error responses
	for i := 0; i < 2; i++ {
		violations := v.Validate(types.NetworkBody{
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
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Response with new field 'created_at'
	violations := v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","created_at":"2025-01-20T14:30:00Z"}`,
		ContentType:  "application/json",
	})

	found := false
	for _, v := range violations {
		if v.ViolationType == "new_field" && containsField(v.NewFields, "created_at") {
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
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"avatar":"https://example.com/avatar.png"}`,
			ContentType:  "application/json",
		})
	}

	// Now 'avatar' is null
	violations := v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"avatar":null}`,
		ContentType:  "application/json",
	})

	found := false
	for _, v := range violations {
		if v.ViolationType == "null_field" && v.Field == "avatar" {
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
		v.Learn(types.NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	violations := v.Validate(types.NetworkBody{
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
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	})
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob","email":"bob@example.com"}`,
		ContentType:  "application/json",
	})

	// Missing field should not be a violation yet (still learning)
	violations := v.Validate(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/3",
		Status:       200,
		ResponseBody: `{"id":3,"name":"Carol"}`,
		ContentType:  "application/json",
	})

	// During learning phase, should not flag violations
	for _, v := range violations {
		if v.ViolationType == "shape_change" {
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
	v.Learn(types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Error response with different shape
	v.Learn(types.NetworkBody{
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
