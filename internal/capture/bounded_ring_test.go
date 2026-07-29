// bounded_ring_test.go — Regression tests for allocation-free bounded capture storage.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "testing"

func TestBoundedRingOverwritesOldestAndPreservesOrder(t *testing.T) {
	ring := newBoundedRing[int](3)
	for value := 1; value <= 5; value++ {
		ring.push(value)
	}

	got := ring.snapshot()
	want := []int{3, 4, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("snapshot[%d] = %d, want %d (snapshot=%v)", i, got[i], want[i], got)
		}
	}
}

func TestBoundedRingSteadyStatePushDoesNotAllocate(t *testing.T) {
	ring := newBoundedRing[int](8)
	for value := 0; value < ring.capacity(); value++ {
		ring.push(value)
	}

	allocs := testing.AllocsPerRun(1000, func() {
		ring.push(9)
	})
	if allocs != 0 {
		t.Fatalf("steady-state push allocated %.2f times per run, want 0", allocs)
	}
}

func TestBoundedRingDropOldestReleasesSlots(t *testing.T) {
	ring := newBoundedRing[*int](4)
	values := []int{1, 2, 3, 4}
	for i := range values {
		ring.push(&values[i])
	}

	ring.dropOldest(3)
	if ring.len() != 1 || **ring.at(0) != 4 {
		t.Fatalf("after drop: len=%d snapshot=%v", ring.len(), ring.snapshot())
	}
	for i := 0; i < 3; i++ {
		if ring.storage[i] != nil {
			t.Fatalf("evicted storage slot %d retained a pointer", i)
		}
	}
}
