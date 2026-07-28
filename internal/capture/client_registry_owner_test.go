// client_registry_owner_test.go — Registry-owner synchronization tests.
package capture

import (
	"sync"
	"testing"
)

func TestClientRegistryOwnerAllowsConcurrentReplacementAndRead(t *testing.T) {
	t.Parallel()

	owner := newClientRegistryOwner()
	registry := clientRegistryStub{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			owner.Set(registry)
		}()
		go func() {
			defer wg.Done()
			_ = owner.Registry()
		}()
	}
	wg.Wait()

	if got := owner.Registry(); got != registry {
		t.Fatalf("Registry() = %#v, want installed registry", got)
	}
}

type clientRegistryStub struct{}

func (clientRegistryStub) Count() int             { return 0 }
func (clientRegistryStub) List() any              { return nil }
func (clientRegistryStub) Register(string) any    { return nil }
func (clientRegistryStub) Get(string) any         { return nil }
func (clientRegistryStub) Unregister(string) bool { return false }
