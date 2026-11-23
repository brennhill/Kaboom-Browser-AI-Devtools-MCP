// context_roots_test.go — Tests that every internal package sits under a declared bounded-context root.
package main

import (
	"strings"
	"testing"
)

func TestContextRootViolationsFlagsUndeclaredRoot(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":                          "module " + strings.TrimSuffix(modulePrefix, "/") + "\n",
		"internal/observation/store/s.go": "package store\n",
		"internal/misc/helpers/h.go":      "package helpers\n",
		"cmd/browser-agent/main.go":       "package main\n",
	})
	scan, err := scanTree(root)
	if err != nil {
		t.Fatal(err)
	}
	got := contextRootViolations(scan)
	if len(got) != 1 || !strings.Contains(got[0], "internal/misc/helpers") {
		t.Fatalf("contextRootViolations = %v, want one violation for internal/misc/helpers", got)
	}
}

func TestRepositoryHasNoUndeclaredContextRoots(t *testing.T) {
	scan, err := scanTree(repoRoot("."))
	if err != nil {
		t.Fatal(err)
	}
	if got := contextRootViolations(scan); len(got) > 0 {
		t.Fatalf("undeclared context roots:\n%s", strings.Join(got, "\n"))
	}
}
