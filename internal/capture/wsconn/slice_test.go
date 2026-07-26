// Purpose: Tests the order-preserving slice removal helper used by connection LRU bookkeeping.
// Docs: docs/features/feature/observe/index.md

package wsconn

import "testing"

// ============================================
// removeFromSlice Tests
// ============================================

func TestNewRemoveFromSlice_RemovesFirstOccurrence(t *testing.T) {
	t.Parallel()

	input := []string{"a", "b", "c", "b", "d"}
	result := removeFromSlice(input, "b")

	want := []string{"a", "c", "b", "d"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_RemovesFirst(t *testing.T) {
	t.Parallel()

	input := []string{"x", "y", "z"}
	result := removeFromSlice(input, "x")

	want := []string{"y", "z"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice(first) len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice(first)[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_RemovesLast(t *testing.T) {
	t.Parallel()

	input := []string{"x", "y", "z"}
	result := removeFromSlice(input, "z")

	want := []string{"x", "y"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice(last) len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice(last)[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_ItemNotFound(t *testing.T) {
	t.Parallel()

	input := []string{"a", "b", "c"}
	result := removeFromSlice(input, "missing")

	// Should return original slice unchanged
	if len(result) != 3 {
		t.Fatalf("removeFromSlice(not found) len = %d, want 3", len(result))
	}
	for i := range result {
		if result[i] != input[i] {
			t.Errorf("removeFromSlice(not found)[%d] = %q, want %q", i, result[i], input[i])
		}
	}
}

func TestNewRemoveFromSlice_SingleElement(t *testing.T) {
	t.Parallel()

	input := []string{"only"}
	result := removeFromSlice(input, "only")

	if len(result) != 0 {
		t.Fatalf("removeFromSlice(single) len = %d, want 0", len(result))
	}
}

func TestNewRemoveFromSlice_EmptySlice(t *testing.T) {
	t.Parallel()

	input := []string{}
	result := removeFromSlice(input, "anything")

	if len(result) != 0 {
		t.Fatalf("removeFromSlice(empty) len = %d, want 0", len(result))
	}
}

func TestNewRemoveFromSlice_PreservesOrder(t *testing.T) {
	t.Parallel()

	input := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	result := removeFromSlice(input, "gamma")

	want := []string{"alpha", "beta", "delta", "epsilon"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice(order) len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice(order)[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_NewBackingArray(t *testing.T) {
	t.Parallel()

	// Verify that removeFromSlice allocates a new backing array (no GC pinning)
	input := []string{"a", "b", "c"}
	result := removeFromSlice(input, "b")

	// Modifying result should not affect input
	if len(result) > 0 {
		result[0] = "modified"
	}
	if input[0] != "a" {
		t.Error("removeFromSlice should allocate new backing array; modifying result affected input")
	}
}
