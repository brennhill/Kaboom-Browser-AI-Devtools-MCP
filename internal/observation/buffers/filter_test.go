// filter_test.go — Tests for ReverseFilterLimit generic helper.

package buffers

import (
	"testing"
)

func TestReverseFilterLimit_EmptySlice(t *testing.T) {
	result := ReverseFilterLimit([]int{}, func(i int) bool { return true }, 10)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d elements", len(result))
	}
}

func TestReverseFilterLimit_AllMatch(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	result := ReverseFilterLimit(input, func(i int) bool { return true }, 10)
	if len(result) != 5 {
		t.Fatalf("expected 5 results, got %d", len(result))
	}
	// Verify newest-first order (reversed)
	expected := []int{5, 4, 3, 2, 1}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestReverseFilterLimit_NoneMatch(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	result := ReverseFilterLimit(input, func(i int) bool { return false }, 10)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d elements", len(result))
	}
}

func TestReverseFilterLimit_LimitReached(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	result := ReverseFilterLimit(input, func(i int) bool { return true }, 3)
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	// Should get the 3 newest (from the end)
	expected := []int{5, 4, 3}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestReverseFilterLimit_ZeroLimitReturnsAll(t *testing.T) {
	input := []int{10, 20, 30}
	result := ReverseFilterLimit(input, func(i int) bool { return true }, 0)
	if len(result) != 3 {
		t.Fatalf("expected 3 results with limit=0, got %d", len(result))
	}
	expected := []int{30, 20, 10}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestReverseFilterLimit_NegativeLimitReturnsAll(t *testing.T) {
	input := []int{10, 20, 30}
	result := ReverseFilterLimit(input, func(i int) bool { return true }, -1)
	if len(result) != 3 {
		t.Fatalf("expected 3 results with limit=-1, got %d", len(result))
	}
}

func TestReverseFilterLimit_NewestFirstOrdering(t *testing.T) {
	// Simulate time-ordered entries where index = time order
	type entry struct {
		id   int
		keep bool
	}
	input := []entry{
		{id: 1, keep: false},
		{id: 2, keep: true},
		{id: 3, keep: false},
		{id: 4, keep: true},
		{id: 5, keep: true},
	}
	result := ReverseFilterLimit(input, func(e entry) bool { return e.keep }, 10)
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	// Newest first: 5, 4, 2
	expectedIDs := []int{5, 4, 2}
	for i, e := range result {
		if e.id != expectedIDs[i] {
			t.Errorf("result[%d].id = %d, want %d", i, e.id, expectedIDs[i])
		}
	}
}

func TestReverseFilterLimit_LimitWithFilter(t *testing.T) {
	// Only even numbers, limit 2
	input := []int{1, 2, 3, 4, 5, 6, 7, 8}
	result := ReverseFilterLimit(input, func(i int) bool { return i%2 == 0 }, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	// Newest even numbers: 8, 6
	expected := []int{8, 6}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestReverseFilterLimit_MapType(t *testing.T) {
	// Test with map[string]any to mirror real usage
	input := []map[string]any{
		{"level": "info", "msg": "a"},
		{"level": "error", "msg": "b"},
		{"level": "info", "msg": "c"},
		{"level": "error", "msg": "d"},
	}
	result := ReverseFilterLimit(input, func(e map[string]any) bool {
		level, _ := e["level"].(string)
		return level == "error"
	}, 10)
	if len(result) != 2 {
		t.Fatalf("expected 2 error entries, got %d", len(result))
	}
	if result[0]["msg"] != "d" {
		t.Errorf("first result msg = %v, want 'd'", result[0]["msg"])
	}
	if result[1]["msg"] != "b" {
		t.Errorf("second result msg = %v, want 'b'", result[1]["msg"])
	}
}
