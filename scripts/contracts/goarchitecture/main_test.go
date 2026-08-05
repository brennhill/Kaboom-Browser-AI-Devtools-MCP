// main_test.go — Pins deterministic Go architecture ratchets.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateRejectsMutableGlobalAndExportGrowth(t *testing.T) {
	baseline := inventory{"pkg/state.go": {MutableGlobals: 1, Exports: 2}}
	current := inventory{"pkg/state.go": {MutableGlobals: 2, Exports: 3}}
	violations := evaluate(current, baseline)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want mutable-global and export growth", violations)
	}
}

func TestEvaluateAllowsReductionsAndDeletedFiles(t *testing.T) {
	baseline := inventory{
		"pkg/state.go": {MutableGlobals: 2, Exports: 3},
		"pkg/old.go":   {MutableGlobals: 1, Exports: 1},
	}
	current := inventory{"pkg/state.go": {MutableGlobals: 1, Exports: 2}}
	if violations := evaluate(current, baseline); len(violations) != 0 {
		t.Fatalf("reduction violations = %v", violations)
	}
}

func TestEvaluateAllowsExportsToMoveWithinTheirOwningPackage(t *testing.T) {
	baseline := inventory{
		"pkg/first.go":  {Exports: 2},
		"pkg/second.go": {Exports: 1},
	}
	current := inventory{
		"pkg/canonical.go": {Exports: 3},
	}
	if violations := evaluate(current, baseline); len(violations) != 0 {
		t.Fatalf("same-package move violations = %v", violations)
	}
}

func TestScanSeparatesMutableGlobalsFromExportedSurface(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pkg", "state.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package pkg
const Limit = 3
var state = map[string]int{}
var ErrSentinel = errors.New("sentinel")
func Public() {}
func private() {}
type Contract struct{}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := got["pkg/state.go"]
	if entry.MutableGlobals != 1 {
		t.Fatalf("mutable globals = %d, want 1 (sentinel errors are classified immutable)", entry.MutableGlobals)
	}
	if entry.Exports != 4 {
		t.Fatalf("exports = %d, want 4", entry.Exports)
	}
}
