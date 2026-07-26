// Purpose: Coverage-expansion tests for error clustering and API schema analysis edge cases and branch paths.
// Docs: docs/features/feature/api-schema/index.md

// branch_coverage_test.go — Branch coverage tests for partially-covered functions.
package analysis

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// describeType — covers $array map and default branches
// ============================================

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

// ============================================
// toStringMap — covers nested map and default branches
// ============================================

func TestToStringMap(t *testing.T) {
	t.Run("string values pass through", func(t *testing.T) {
		m := map[string]any{"name": "alice"}
		got := toStringMap(m)
		if got["name"] != "alice" {
			t.Errorf("got %v, want alice", got["name"])
		}
	})

	t.Run("nested maps are recursed", func(t *testing.T) {
		m := map[string]any{
			"user": map[string]any{"name": "bob"},
		}
		got := toStringMap(m)
		nested, ok := got["user"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested map, got %T", got["user"])
		}
		if nested["name"] != "bob" {
			t.Errorf("got %v, want bob", nested["name"])
		}
	})

	t.Run("non-string values use describeType", func(t *testing.T) {
		m := map[string]any{
			"count": 42,
			"nil":   nil,
		}
		got := toStringMap(m)
		if got["count"] != "int" {
			t.Errorf("count = %v, want int", got["count"])
		}
		if got["nil"] != "null" {
			t.Errorf("nil = %v, want null", got["nil"])
		}
	})
}

// ============================================
// isJSONContentType — covers empty string branch
// ============================================

func TestIsJSONContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/html", false},
		{"text/plain", false},
		{"application/xml", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isJSONContentType(tt.input); got != tt.want {
				t.Errorf("isJSONContentType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================
// recordStatusOnly — covers zero status, new status, existing status, cap
// ============================================

func TestRecordStatusOnly(t *testing.T) {
	t.Run("zero status is ignored", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, 0)
		if acc.responseShapes != nil {
			t.Error("expected nil responseShapes for zero status")
		}
	})

	t.Run("negative status is ignored", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, -1)
		if acc.responseShapes != nil {
			t.Error("expected nil responseShapes for negative status")
		}
	})

	t.Run("new status creates shape", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, 200)
		if acc.responseShapes == nil {
			t.Fatal("expected responseShapes to be initialized")
		}
		if acc.responseShapes[200] == nil {
			t.Fatal("expected shape for status 200")
		}
		if acc.responseShapes[200].count != 1 {
			t.Errorf("count = %d, want 1", acc.responseShapes[200].count)
		}
	})

	t.Run("existing status increments count", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		s.recordStatusOnly(acc, 200)
		s.recordStatusOnly(acc, 200)
		s.recordStatusOnly(acc, 200)
		if acc.responseShapes[200].count != 3 {
			t.Errorf("count = %d, want 3", acc.responseShapes[200].count)
		}
	})

	t.Run("cap prevents new shapes beyond limit", func(t *testing.T) {
		s := NewSchemaStore()
		acc := &endpointAccumulator{}
		// Fill to maxResponseShapes
		for i := 1; i <= maxResponseShapes; i++ {
			s.recordStatusOnly(acc, i)
		}
		beforeCount := len(acc.responseShapes)
		// Try adding one more
		s.recordStatusOnly(acc, maxResponseShapes+1)
		if len(acc.responseShapes) != beforeCount {
			t.Errorf("expected cap at %d, got %d", beforeCount, len(acc.responseShapes))
		}
	})
}

// ============================================
// EndpointCount
// ============================================

func TestEndpointCount(t *testing.T) {
	s := NewSchemaStore()
	if s.EndpointCount() != 0 {
		t.Errorf("expected 0 endpoints, got %d", s.EndpointCount())
	}

	// Observe a body to create an endpoint
	s.Observe(capture.NetworkBody{
		Method:       "GET",
		URL:          "https://api.example.com/users",
		Status:       200,
		ResponseBody: `{"id": 1}`,
		ContentType:  "application/json",
	})

	if s.EndpointCount() != 1 {
		t.Errorf("expected 1 endpoint, got %d", s.EndpointCount())
	}
}

// ============================================
// recordObservation — covers first call, subsequent calls, history cap
// ============================================

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
		time.Sleep(time.Millisecond)
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
		// Newest should be at the end
		last := tracker.StatusHistory[len(tracker.StatusHistory)-1]
		if last != 200+maxStatusHistory+4 {
			t.Errorf("last status = %d, want %d", last, 200+maxStatusHistory+4)
		}
	})
}
