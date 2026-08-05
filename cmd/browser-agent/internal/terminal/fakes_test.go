// fakes_test.go -- Shared test doubles for terminal handler tests.
// Provides in-memory fakes for ServerDeps, IntentDeps, RelayMap, and the
// capture ClientRegistry so handler logic can be exercised without real
// servers, PTYs, or MCP clients.

package terminal

// fakeServerDeps is an in-memory ServerDeps for testing active-codebase logic.
type fakeServerDeps struct {
	codebase string
}

func (f *fakeServerDeps) GetActiveCodebase() string  { return f.codebase }
func (f *fakeServerDeps) SetActiveCodebase(p string) { f.codebase = p }

// fakeRelayMap is an in-memory RelayMap capturing injected writes.
type fakeRelayMap struct {
	writeOK   bool
	written   [][]byte
	closedAll bool
}

func (f *fakeRelayMap) WriteToFirst(data []byte) bool {
	cp := make([]byte, len(data))
	copy(cp, data)
	f.written = append(f.written, cp)
	return f.writeOK
}

func (f *fakeRelayMap) CloseAll() { f.closedAll = true }

// fakeIntentDeps is an in-memory IntentDeps. Either dependency may be nil to
// exercise the "not initialized" error branches.
type fakeIntentDeps struct {
	relays RelayMap
	store  *IntentStore
}

func (f *fakeIntentDeps) GetPtyRelays() RelayMap       { return f.relays }
func (f *fakeIntentDeps) GetIntentStore() *IntentStore { return f.store }

// fakeClientRegistry implements clientstore.Registry for AutoDetectCWD tests.
// listResult is returned verbatim from List(); its concrete type selects which
// branch of AutoDetectCWD executes ([]any fast path vs. JSON-roundtrip default).
type fakeClientRegistry struct {
	listResult any
}

func (f *fakeClientRegistry) Count() int                { return 0 }
func (f *fakeClientRegistry) List() any                 { return f.listResult }
func (f *fakeClientRegistry) Register(cwd string) any   { return nil }
func (f *fakeClientRegistry) Get(id string) any         { return nil }
func (f *fakeClientRegistry) Unregister(id string) bool { return false }
