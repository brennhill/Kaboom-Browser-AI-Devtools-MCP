// store_test.go — Verifies bounded FIFO retention and detached snapshots.
package ringstore

import "testing"

func TestStoreEvictsOldestAndPreservesFIFOOrder(t *testing.T) {
	store := New[int](2)
	if _, overwritten := store.Push(1); overwritten {
		t.Fatal("first push unexpectedly overwrote an entry")
	}
	store.Push(2)
	evicted, overwritten := store.Push(3)
	if !overwritten || evicted != 1 {
		t.Fatalf("Push(3) = (%d, %v), want (1, true)", evicted, overwritten)
	}
	if got := store.Snapshot(); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("Snapshot() = %v, want [2 3]", got)
	}
}

func TestStoreNormalizesNegativeCapacityAndDropsWithoutLooping(t *testing.T) {
	store := New[int](-1)
	if store.Capacity() != 0 {
		t.Fatalf("Capacity() = %d, want 0", store.Capacity())
	}
	if evicted, overwritten := store.Push(7); evicted != 7 || !overwritten {
		t.Fatalf("zero-capacity Push(7) = (%d, %v), want (7, true)", evicted, overwritten)
	}

	store = New[int](3)
	store.Push(1)
	store.Push(2)
	store.Push(3)
	store.DropOldest(-1)
	if got := store.Snapshot(); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("Snapshot() after negative DropOldest = %v, want [1 2 3]", got)
	}
	store.DropOldest(2)
	if got := store.Snapshot(); len(got) != 1 || got[0] != 3 {
		t.Fatalf("Snapshot() after DropOldest = %v, want [3]", got)
	}
	store.Clear()
	if store.Len() != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", store.Len())
	}
}

func TestStoreSteadyStatePushDoesNotAllocate(t *testing.T) {
	store := New[int](8)
	for value := 0; value < store.Capacity(); value++ {
		store.Push(value)
	}
	allocs := testing.AllocsPerRun(1000, func() {
		store.Push(9)
	})
	if allocs != 0 {
		t.Fatalf("steady-state Push allocated %.2f times per run, want 0", allocs)
	}
}

func TestStoreDropOldestReleasesPointerSlots(t *testing.T) {
	store := New[*int](4)
	values := []int{1, 2, 3, 4}
	for i := range values {
		store.Push(&values[i])
	}
	store.DropOldest(3)
	if store.Len() != 1 || **store.At(0) != 4 {
		t.Fatalf("after drop: len=%d snapshot=%v", store.Len(), store.Snapshot())
	}
	for i := 0; i < 3; i++ {
		if store.storage[i] != nil {
			t.Fatalf("evicted storage slot %d retained a pointer", i)
		}
	}
}
