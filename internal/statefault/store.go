// store.go — Canonical deterministic fault wrapper for key-value persistence tests.
package statefault

// Store is the minimal persisted key-value boundary shared by daemon state owners.
type Store interface {
	Save(namespace, key string, data []byte) error
	Load(namespace, key string) ([]byte, error)
	List(namespace string) ([]string, error)
	Delete(namespace, key string) error
}

type storeFixture struct {
	base     Store
	scenario Scenario
}

// NewStore wraps a real in-memory or temporary-directory store with one fault.
func NewStore(base Store, scenario Scenario) Store {
	return &storeFixture{base: base, scenario: scenario}
}

func (fixture *storeFixture) Save(namespace, key string, data []byte) error {
	if fixture.scenario.kind == Cancellation || isWriteFault(fixture.scenario.kind) {
		return fixture.scenario.Error()
	}
	return fixture.base.Save(namespace, key, data)
}

func (fixture *storeFixture) Load(namespace, key string) ([]byte, error) {
	if fixture.scenario.kind == Read || fixture.scenario.kind == Cancellation {
		return nil, fixture.scenario.Error()
	}
	data, err := fixture.base.Load(namespace, key)
	if err != nil {
		return nil, err
	}
	return fixture.scenario.Payload(data), nil
}

func (fixture *storeFixture) List(namespace string) ([]string, error) {
	if fixture.scenario.kind == Read || fixture.scenario.kind == Cancellation {
		return nil, fixture.scenario.Error()
	}
	return fixture.base.List(namespace)
}

func (fixture *storeFixture) Delete(namespace, key string) error {
	if fixture.scenario.kind == Cancellation || isWriteFault(fixture.scenario.kind) {
		return fixture.scenario.Error()
	}
	return fixture.base.Delete(namespace, key)
}

func isWriteFault(kind Kind) bool {
	switch kind {
	case Write, Sync, Rename, DirectorySync, Quota:
		return true
	default:
		return false
	}
}
