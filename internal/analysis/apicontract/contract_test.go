// contract_test.go — Tests API schema learning, normalization, and violation detection.
// Docs: docs/features/feature/api-schema/index.md

// Tests schema learning, shape comparison, violation detection, and the MCP tool interface.
// Design: TDD approach - tests written first to define expected behavior.
package apicontract

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestAPIContractPackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("apicontract package has %d files; want at most 10 change-coupled owners", files)
	}
}

func TestCompareShapesReportsTopLevelTypeChanges(t *testing.T) {
	t.Parallel()
	validator := &APIContractValidator{}
	if violations := validator.compareShapes("GET /items", []any{}, []any{}, nil); len(violations) != 0 {
		t.Fatalf("matching top-level types = %#v", violations)
	}
	violations := validator.compareShapes("GET /items", map[string]any{"items": "array"}, []any{}, nil)
	if len(violations) != 1 || violations[0].ViolationType != "type_change" || violations[0].ExpectedType == violations[0].ActualType {
		t.Fatalf("top-level type change = %#v", violations)
	}
}

func TestDescribeType(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"nil", nil, "null"},
		{"string type", "integer", "integer"},
		{"plain object", map[string]any{"key": "val"}, "object"},
		{"array marker", map[string]any{"$array": true}, "array"},
		{"int fallback", 42, "int"},
		{"float fallback", 3.14, "float64"},
		{"bool fallback", true, "bool"},
		{"slice fallback", []string{"a"}, "[]string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeType(tt.input)
			if got != tt.want {
				t.Errorf("describeType(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToStringMap(t *testing.T) {
	t.Run("string values pass through", func(t *testing.T) {
		got := toStringMap(map[string]any{"name": "alice"})
		if got["name"] != "alice" {
			t.Errorf("got %v, want alice", got["name"])
		}
	})

	t.Run("nested maps are recursed", func(t *testing.T) {
		got := toStringMap(map[string]any{"user": map[string]any{"name": "bob"}})
		nested, ok := got["user"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested map, got %T", got["user"])
		}
		if nested["name"] != "bob" {
			t.Errorf("got %v, want bob", nested["name"])
		}
	})

	t.Run("non-string values use describeType", func(t *testing.T) {
		got := toStringMap(map[string]any{"count": 42, "nil": nil})
		if got["count"] != "int" {
			t.Errorf("count = %v, want int", got["count"])
		}
		if got["nil"] != "null" {
			t.Errorf("nil = %v, want null", got["nil"])
		}
	})
}

func TestRecordObservation(t *testing.T) {
	v := NewAPIContractValidator()

	t.Run("first call sets FirstCalled", func(t *testing.T) {
		tracker := &EndpointTracker{}
		v.recordObservation(tracker, 200)
		if tracker.FirstCalled.IsZero() {
			t.Error("FirstCalled should be set")
		}
		if tracker.CallCount != 1 {
			t.Errorf("CallCount = %d, want 1", tracker.CallCount)
		}
		if len(tracker.StatusHistory) != 1 || tracker.StatusHistory[0] != 200 {
			t.Errorf("StatusHistory = %v, want [200]", tracker.StatusHistory)
		}
	})

	t.Run("subsequent calls do not reset FirstCalled", func(t *testing.T) {
		tracker := &EndpointTracker{}
		v.recordObservation(tracker, 200)
		first := tracker.FirstCalled
		v.recordObservation(tracker, 201)
		if !tracker.FirstCalled.Equal(first) {
			t.Error("FirstCalled should not change after first call")
		}
		if tracker.CallCount != 2 {
			t.Errorf("CallCount = %d, want 2", tracker.CallCount)
		}
	})

	t.Run("status history caps at maxStatusHistory", func(t *testing.T) {
		tracker := &EndpointTracker{}
		for i := 0; i < maxStatusHistory+5; i++ {
			v.recordObservation(tracker, 200+i)
		}
		if len(tracker.StatusHistory) != maxStatusHistory {
			t.Errorf("StatusHistory len = %d, want %d", len(tracker.StatusHistory), maxStatusHistory)
		}
		last := tracker.StatusHistory[len(tracker.StatusHistory)-1]
		if last != 200+maxStatusHistory+4 {
			t.Errorf("last status = %d, want %d", last, 200+maxStatusHistory+4)
		}
	})
}

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

	learned := []types.NetworkBody{
		{Method: "GET", URL: "http://localhost:3000/api/users/1", Status: 200,
			ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`, ContentType: "application/json"},
		{Method: "GET", URL: "http://localhost:3000/api/users/2", Status: 200,
			ResponseBody: `{"id":2,"name":"Bob","email":"bob@example.com"}`, ContentType: "application/json"},
		{Method: "GET", URL: "http://localhost:3000/api/users/4", Status: 200,
			ResponseBody: `{"id":4,"name":"Dave","email":"dave@example.com"}`, ContentType: "application/json"},
	}
	// The same probe drives both arms: "email" is absent from the learned shape.
	probe := types.NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/3",
		Status:       200,
		ResponseBody: `{"id":3,"name":"Carol"}`,
		ContentType:  "application/json",
	}

	// Discriminating control: once the shape IS established the very same probe
	// must be flagged. Without this arm the assertion below would hold equally
	// well if Validate never inspected the body at all.
	established := NewAPIContractValidator()
	for _, body := range learned {
		established.Learn(body)
	}
	controlViolations := established.Validate(probe)
	controlFlagged := false
	for _, violation := range controlViolations {
		if violation.ViolationType == "shape_change" && containsField(violation.MissingFields, "email") {
			controlFlagged = true
			break
		}
	}
	if !controlFlagged {
		t.Fatalf("control: expected shape_change for missing 'email' after %d observations, got %+v",
			minCallsToEstablishShape, controlViolations)
	}

	// Subject: only minCallsToEstablishShape-1 observations, so the shape is
	// still being learned and the identical probe must NOT be flagged.
	v := NewAPIContractValidator()
	for _, body := range learned[:minCallsToEstablishShape-1] {
		v.Learn(body)
	}
	violations := v.Validate(probe)
	for _, violation := range violations {
		if violation.ViolationType == "shape_change" {
			t.Errorf("Should not report shape_change violation before shape is established (min %d calls), got %+v",
				minCallsToEstablishShape, violation)
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
