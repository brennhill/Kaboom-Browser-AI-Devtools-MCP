// owner_test.go — Defines deterministic client-registry ownership contracts.
package clientstore

import (
	"sync"
	"testing"
)

func TestOwnerAllowsConcurrentReplacementAndRead(t *testing.T) {
	t.Parallel()

	owner := New()
	registry := registryStub{}
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

type registryStub struct{}

func (registryStub) Count() int             { return 0 }
func (registryStub) List() any              { return nil }
func (registryStub) Register(string) any    { return nil }
func (registryStub) Get(string) any         { return nil }
func (registryStub) Unregister(string) bool { return false }
