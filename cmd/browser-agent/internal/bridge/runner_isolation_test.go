// runner_isolation_test.go — Regression coverage for instance-scoped collaborators.
package bridge

import (
	"testing"
)

func TestRunnerDependenciesAreInstanceScoped(t *testing.T) {
	first := newTestRunner()
	second := newTestRunner()
	first.identity.Version = "first"
	if second.identity.Version == first.identity.Version {
		t.Fatal("runner identity leaked across constructed instances")
	}
}
