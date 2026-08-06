// store_test.go — Verifies synchronized active-codebase state ownership.
// Docs: docs/features/feature/terminal/index.md

package activecodebase

import (
	"sync"
	"testing"
)

func TestStoreSetAndGet(t *testing.T) {
	t.Parallel()
	store := New()
	if got := store.GetActiveCodebase(); got != "" {
		t.Fatalf("initial codebase = %q, want empty", got)
	}
	store.SetActiveCodebase("/workspace/app")
	if got := store.GetActiveCodebase(); got != "/workspace/app" {
		t.Fatalf("codebase = %q", got)
	}
}

func TestStoreSupportsConcurrentReadersAndWriters(t *testing.T) {
	t.Parallel()
	store := New()
	var group sync.WaitGroup
	for index := 0; index < 20; index++ {
		group.Add(2)
		go func() {
			defer group.Done()
			store.SetActiveCodebase("/workspace/app")
		}()
		go func() {
			defer group.Done()
			_ = store.GetActiveCodebase()
		}()
	}
	group.Wait()
	if got := store.GetActiveCodebase(); got != "/workspace/app" {
		t.Fatalf("final codebase = %q", got)
	}
}
