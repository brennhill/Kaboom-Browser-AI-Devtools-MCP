// Purpose: Tests for bridge context cancellation contract compliance.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package bridge

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// findFuncDecl locates a top-level func by name anywhere in this package's
// sources. It deliberately does NOT name a file: pinning a source-contract test
// to a filename makes the contract silently unenforceable the moment the file is
// renamed or merged, which is the opposite of what a contract test is for.
func findFuncDecl(t *testing.T, name string) (*ast.FuncDecl, *token.FileSet) {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		fileName := e.Name()
		if e.IsDir() || !strings.HasSuffix(fileName, ".go") || strings.HasSuffix(fileName, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, fileName, nil, 0)
		if parseErr != nil {
			t.Fatalf("failed to parse %s: %v", fileName, parseErr)
		}
		for _, decl := range file.Decls {
			d, ok := decl.(*ast.FuncDecl)
			if ok && d.Name.Name == name {
				return d, fset
			}
		}
	}
	t.Fatalf("%s not found in any source file of package bridge", name)
	return nil, nil
}

// TestBridgeForwardRequest_NoCancelReassignment enforces a safety contract:
// the cancel func created with context.WithTimeout must not be reassigned.
// Reassignment after defer cancel() can leave a later timeout context uncanceled.
func TestBridgeForwardRequest_NoCancelReassignment(t *testing.T) {
	t.Parallel()

	fn, _ := findFuncDecl(t, "bridgeForwardRequest")

	reassigned := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Tok != token.ASSIGN {
			return true
		}
		for _, lhs := range assign.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "cancel" {
				reassigned = true
				return false
			}
		}
		return true
	})

	if reassigned {
		t.Fatal("bridgeForwardRequest reassigns cancel; create a new cancel variable for retry context")
	}
}

func TestBridgeForwardRequest_NoRetryWithCtxCancelAssignmentPattern(t *testing.T) {
	t.Parallel()

	fn, fset := findFuncDecl(t, "bridgeForwardRequest")

	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		var b strings.Builder
		if err := format.Node(&b, fset, assign); err != nil {
			t.Fatalf("failed to render assignment: %v", err)
		}
		if strings.Contains(b.String(), "ctx, cancel = context.WithTimeout") {
			t.Fatalf("found fragile retry pattern %q; must use a fresh retry cancel variable", b.String())
		}
	}
}
