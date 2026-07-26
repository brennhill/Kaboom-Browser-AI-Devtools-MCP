// Purpose: Coverage-expansion tests for API contract validation edge cases and branch paths.
// Docs: docs/features/feature/api-schema/index.md

package analysis

import (
	"testing"
	"time"
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
