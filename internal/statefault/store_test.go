// store_test.go — Proves canonical key-value persistence fault behavior.
package statefault

import (
	"errors"
	"reflect"
	"testing"
)

type memoryStore map[string][]byte

func (store memoryStore) Save(namespace, key string, data []byte) error {
	store[namespace+"/"+key] = append([]byte(nil), data...)
	return nil
}

func (store memoryStore) Load(namespace, key string) ([]byte, error) {
	data, found := store[namespace+"/"+key]
	if !found {
		return nil, errors.New("absent")
	}
	return append([]byte(nil), data...), nil
}

func (store memoryStore) List(namespace string) ([]string, error) { return []string{"item"}, nil }
func (store memoryStore) Delete(namespace, key string) error {
	delete(store, namespace+"/"+key)
	return nil
}

func TestStoreFixtureMapsEveryFaultToStableBoundaryBehavior(t *testing.T) {
	const private = "private-store-value"
	valid := []byte(`{"value":"private-store-value"}`)
	for _, kind := range Kinds() {
		t.Run(string(kind), func(t *testing.T) {
			base := memoryStore{"state/item": valid}
			fixture := NewStore(base, New(kind, private))
			loaded, loadErr := fixture.Load("state", "item")
			list, listErr := fixture.List("state")
			saveErr := fixture.Save("state", "next", valid)
			deleteErr := fixture.Delete("state", "item")

			switch kind {
			case Read:
				assertInjected(t, loadErr)
				assertInjected(t, listErr)
			case Corruption, PartialWrite:
				if !reflect.DeepEqual(loaded, New(kind, private).Payload(valid)) {
					t.Fatalf("loaded payload = %q, want canonical %s payload", loaded, kind)
				}
			case Write, Sync, Rename, DirectorySync, Quota:
				assertInjected(t, saveErr)
				assertInjected(t, deleteErr)
			case Cancellation:
				assertInjected(t, loadErr)
				assertInjected(t, listErr)
				assertInjected(t, saveErr)
				assertInjected(t, deleteErr)
			case Restart:
				if loadErr != nil || listErr != nil || saveErr != nil || deleteErr != nil || len(list) != 1 {
					t.Fatal("restart fixture must preserve durable store behavior")
				}
			}
		})
	}
}

func assertInjected(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInjected) {
		t.Fatalf("error = %v, want ErrInjected", err)
	}
}
