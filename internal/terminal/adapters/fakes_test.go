// fakes_test.go -- Shared test doubles for terminal handler tests.
// Provides an in-memory fake for ServerDeps.

package terminal

// fakeServerDeps is an in-memory ServerDeps for testing active-codebase logic.
type fakeServerDeps struct {
	codebase string
}

func (f *fakeServerDeps) GetActiveCodebase() string  { return f.codebase }
func (f *fakeServerDeps) SetActiveCodebase(p string) { f.codebase = p }
