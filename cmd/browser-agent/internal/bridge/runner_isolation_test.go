// runner_isolation_test.go — Regression coverage for instance-scoped collaborators.
package bridge

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
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

func TestBridgeHasNoGlobalDependencyLocator(t *testing.T) {
	source, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "runner.go", source, 0); err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{"var deps ", "func Init(", "type Deps struct"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("obsolete dependency surface remains: %q", forbidden)
		}
	}
}
