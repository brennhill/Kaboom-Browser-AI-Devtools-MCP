// Purpose: Unit tests for cluster mutation operations (addUnclustered FIFO eviction).
// Docs: docs/features/feature/error-clustering/index.md
package clustering

import (
	"fmt"
	"testing"
	"time"
)

func TestAddUnclustered(t *testing.T) {
	t.Run("below capacity appends", func(t *testing.T) {
		cm := NewClusterManager()
		for i := 0; i < 5; i++ {
			cm.addUnclustered(ErrorInstance{Message: fmt.Sprintf("err-%d", i)})
		}
		if len(cm.unclustered) != 5 {
			t.Fatalf("got %d unclustered, want 5", len(cm.unclustered))
		}
	})

	t.Run("at capacity evicts oldest", func(t *testing.T) {
		cm := NewClusterManager()
		for i := 0; i < 101; i++ {
			cm.addUnclustered(ErrorInstance{Message: fmt.Sprintf("err-%d", i)})
		}
		if len(cm.unclustered) != 100 {
			t.Fatalf("got %d unclustered, want 100", len(cm.unclustered))
		}
		// First item should be err-1 (err-0 was evicted)
		if cm.unclustered[0].Message != "err-1" {
			t.Errorf("first item = %q, want %q", cm.unclustered[0].Message, "err-1")
		}
		if cm.unclustered[99].Message != "err-100" {
			t.Errorf("last item = %q, want %q", cm.unclustered[99].Message, "err-100")
		}
	})

	t.Run("large overflow caps at 100", func(t *testing.T) {
		cm := NewClusterManager()
		for i := 0; i < 200; i++ {
			cm.addUnclustered(ErrorInstance{Message: fmt.Sprintf("err-%d", i)})
		}
		if len(cm.unclustered) != 100 {
			t.Fatalf("got %d unclustered, want 100", len(cm.unclustered))
		}
		// Newest 100 items preserved (err-100 through err-199)
		if cm.unclustered[0].Message != "err-100" {
			t.Errorf("first item = %q, want %q", cm.unclustered[0].Message, "err-100")
		}
		if cm.unclustered[99].Message != "err-199" {
			t.Errorf("last item = %q, want %q", cm.unclustered[99].Message, "err-199")
		}
	})

	t.Run("FIFO ordering preserved", func(t *testing.T) {
		cm := NewClusterManager()
		now := time.Now()
		for i := 0; i < 105; i++ {
			cm.addUnclustered(ErrorInstance{
				Message:   fmt.Sprintf("err-%d", i),
				Timestamp: now.Add(time.Duration(i) * time.Second),
			})
		}
		// Items 0-4 evicted, items 5-104 survive
		for j := 0; j < 100; j++ {
			want := fmt.Sprintf("err-%d", j+5)
			if cm.unclustered[j].Message != want {
				t.Errorf("unclustered[%d].Message = %q, want %q", j, cm.unclustered[j].Message, want)
			}
		}
	})

	t.Run("single item", func(t *testing.T) {
		cm := NewClusterManager()
		cm.addUnclustered(ErrorInstance{Message: "only-one"})
		if len(cm.unclustered) != 1 {
			t.Fatalf("got %d unclustered, want 1", len(cm.unclustered))
		}
		if cm.unclustered[0].Message != "only-one" {
			t.Errorf("got %q, want %q", cm.unclustered[0].Message, "only-one")
		}
	})
}
