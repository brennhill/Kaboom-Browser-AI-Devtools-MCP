// owner_test.go — Defines deterministic client-registry ownership contracts.
package clientstore

import (
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

func TestOwnerAllowsConcurrentReplacementAndRead(t *testing.T) {
	t.Parallel()

	owner := New()
	registry := clientreg.NewClientRegistry()
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 50 {
		workers.Add(2)
		go func() {
			defer workers.Done()
			<-start
			owner.Set(registry)
		}()
		go func() {
			defer workers.Done()
			<-start
			_ = owner.Registry()
		}()
	}
	close(start)
	workers.Wait()

	if got := owner.Registry(); got != registry {
		t.Fatalf("Registry() = %#v, want installed registry", got)
	}
}
