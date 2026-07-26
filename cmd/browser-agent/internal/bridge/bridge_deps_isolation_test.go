// bridge_deps_isolation_test.go -- Guards against the shared-global `deps` clobber.
// initTestDeps replaces the package-global deps installed by TestMain; if it does not
// restore the prior value, it leaks the minimal stub into every test that runs after
// it (this is exactly what broke the FastPath tests once and hung the framed-init test).
package bridge

import (
	"encoding/json"
	"testing"
)

// TestInitTestDepsRestoresGlobalDeps asserts initTestDeps restores the caller's prior
// package-global deps after the test completes, so its stub cannot leak to later tests.
func TestInitTestDepsRestoresGlobalDeps(t *testing.T) {
	// Install a recognizable sentinel so the assertion is independent of whatever
	// earlier tests happened to leave in the package-global deps.
	prev := deps
	sentinel := deps
	sentinel.NegotiateProtocolVersion = func(json.RawMessage) string { return "SENTINEL-VERSION" }
	deps = sentinel
	defer func() { deps = prev }()

	t.Run("inner clobbers deps via initTestDeps", func(t *testing.T) {
		initTestDeps(t)
		if deps.NegotiateProtocolVersion(nil) == "SENTINEL-VERSION" {
			t.Fatal("initTestDeps should have replaced the sentinel deps inside the subtest")
		}
	})

	// t.Run returns only after the subtest's cleanups have run. If initTestDeps
	// registered a restoring t.Cleanup, deps is back to the sentinel; otherwise the
	// stub leaked.
	if deps.NegotiateProtocolVersion(nil) != "SENTINEL-VERSION" {
		t.Fatalf("initTestDeps leaked: it must restore the package-global deps to the caller's prior value")
	}
}
