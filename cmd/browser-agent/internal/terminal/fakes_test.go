// fakes_test.go -- Shared test doubles for terminal handler tests.
// Provides in-memory fakes for ServerDeps and the capture ClientRegistry.

package terminal

// fakeServerDeps is an in-memory ServerDeps for testing active-codebase logic.
type fakeServerDeps struct {
	codebase string
}

func (f *fakeServerDeps) GetActiveCodebase() string  { return f.codebase }
func (f *fakeServerDeps) SetActiveCodebase(p string) { f.codebase = p }

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
